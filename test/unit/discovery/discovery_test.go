package discovery

import (
	"context"
	"encoding/base64"
	configv1alpha1 "github.com/liqotech/liqo/api/config/v1alpha1"
	"github.com/liqotech/liqo/api/discovery/v1alpha1"
	advtypes "github.com/liqotech/liqo/api/sharing/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery"
	foreign_cluster_operator "github.com/liqotech/liqo/internal/discovery/foreign-cluster-operator"
	"github.com/liqotech/liqo/internal/discovery/kubeconfig"
	peering_request_operator "github.com/liqotech/liqo/internal/peering-request-operator"
	"github.com/liqotech/liqo/pkg/crdClient"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"k8s.io/kubectl/pkg/util/slice"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
	"testing"
	"time"
)

var clientCluster *Cluster
var serverCluster *Cluster
var stopChan <-chan struct{}

func setUp() {
	stopChan = ctrl.SetupSignalHandler()

	clientCluster = getClientCluster()
	serverCluster = getServerCluster()

	clientCluster.fcReconciler.ForeignConfig = serverCluster.cfg
	clientCluster.prReconciler.ForeignConfig = serverCluster.cfg
	serverCluster.fcReconciler.ForeignConfig = clientCluster.cfg
	serverCluster.prReconciler.ForeignConfig = clientCluster.cfg

	SetupDNSServer()
}

func tearDown() {
	err := clientCluster.env.Stop()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	err = serverCluster.env.Stop()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}

func TestMain(m *testing.M) {
	setUp()
	defer tearDown()

	// wait cache starting
	time.Sleep(1 * time.Second)

	os.Exit(m.Run())
}

func TestDiscovery(t *testing.T) {
	t.Run("testClient", testClient)
	t.Run("testDiscoveryConfig", testDiscoveryConfig)
	t.Run("testPRConfig", testPRConfig)
	t.Run("testJoin", testJoin)
	t.Run("testUnjoin", testUnjoin)
	t.Run("testBidirectionalJoin", testBidirectionalJoin)
	t.Run("testMergeClusters", testMergeClusters)
	t.Run("testCreateKubeconfig", testCreateKubeconfig)
}

// ------
// tests if environment is correctly set and creation of ForeignCluster with disabled AutoJoin
func testClient(t *testing.T) {
	tmp, err := clientCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err)
	fcs, ok := tmp.(*v1alpha1.ForeignClusterList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(fcs.Items), 0)

	// create new ForeignCluster with disabled AutoJoin
	fc := &v1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fc-test",
		},
		Spec: v1alpha1.ForeignClusterSpec{
			ClusterIdentity: v1alpha1.ClusterIdentity{
				ClusterID: "test-cluster",
			},
			Namespace:     "default",
			Join:          false,
			ApiUrl:        serverCluster.cfg.Host,
			DiscoveryType: v1alpha1.ManualDiscovery,
		},
	}
	_, err = clientCluster.client.Resource("foreignclusters").Create(fc, metav1.CreateOptions{})
	assert.NilError(t, err, "Error during ForeignCluster creation")

	tmp, err = clientCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err)
	fcs, ok = tmp.(*v1alpha1.ForeignClusterList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(fcs.Items), 1, "Number of ForeignCluster on clientCluster is different to 1")

	tmp, err = serverCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err)
	fcs, ok = tmp.(*v1alpha1.ForeignClusterList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(fcs.Items), 0, "Number of ForeignCluster on serverCluster is different to 0, is it crated in wrong cluster?!?")

	tmp, err = serverCluster.client.Resource("peeringrequests").List(metav1.ListOptions{})
	assert.NilError(t, err)
	prs, ok := tmp.(*v1alpha1.PeeringRequestList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(prs.Items), 0, "Peering Request has been created even if join flag was false")

	tmp, err = clientCluster.client.Resource("peeringrequests").List(metav1.ListOptions{})
	assert.NilError(t, err)
	prs, ok = tmp.(*v1alpha1.PeeringRequestList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(prs.Items), 0, "Peering Request has been created in client cluster")
}

// ------
// tests if discovery controller is able to load it's configs from ClusterConfigs
func testDiscoveryConfig(t *testing.T) {
	policyConfig := *clientCluster.cfg
	policyConfig.GroupVersion = &configv1alpha1.GroupVersion
	client, err := crdClient.NewFromConfig(&policyConfig)
	assert.NilError(t, err, "Can't get CRDClient")
	err = clientCluster.discoveryCtrl.GetDiscoveryConfig(client, "")
	assert.NilError(t, err, "DiscoveryCtrl can't load settings")

	tmp, err := client.Resource("clusterconfigs").Get("configuration", metav1.GetOptions{})
	assert.NilError(t, err, "Can't get configurations")
	cc := tmp.(*configv1alpha1.ClusterConfig)
	cc.Spec.DiscoveryConfig.EnableAdvertisement = false
	cc.Spec.DiscoveryConfig.EnableDiscovery = false
	tmp, err = client.Resource("clusterconfigs").Update("configuration", cc, metav1.UpdateOptions{})
	assert.NilError(t, err, "Can't update configurations")
	cc = tmp.(*configv1alpha1.ClusterConfig)

	time.Sleep(1 * time.Second)
	assert.Equal(t, *clientCluster.discoveryCtrl.Config, cc.Spec.DiscoveryConfig)

	cc.Spec.DiscoveryConfig.EnableAdvertisement = true
	cc.Spec.DiscoveryConfig.EnableDiscovery = true
	tmp, err = client.Resource("clusterconfigs").Update("configuration", cc, metav1.UpdateOptions{})
	assert.NilError(t, err, "Can't update configurations")
	cc = tmp.(*configv1alpha1.ClusterConfig)

	time.Sleep(1 * time.Second)
	assert.Equal(t, *clientCluster.discoveryCtrl.Config, cc.Spec.DiscoveryConfig)
}

// ------
// tests if peering request operator is able to load it's configs from configmap
func testPRConfig(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "peering-request-operator-cm",
		},
		Data: map[string]string{
			"allowAll": "true",
		},
	}
	_, err := clientCluster.client.Client().CoreV1().ConfigMaps("default").Create(context.TODO(), cm, metav1.CreateOptions{})
	assert.NilError(t, err, "Unable to create ConfigMaps")
	_, err = peering_request_operator.GetConfig(clientCluster.client, "default")
	assert.NilError(t, err, "PeeringRequest operator can't load settings from ConfigMap")
}

// ------
// tests if enabling Join flag a PeeringRequest and Broadcaster deployment are created on foreign cluster
func testJoin(t *testing.T) {
	tmp, err := clientCluster.client.Resource("foreignclusters").Get("fc-test", metav1.GetOptions{})
	assert.NilError(t, err, "Error retrieving ForeignCluster")
	fc, ok := tmp.(*v1alpha1.ForeignCluster)
	assert.Equal(t, ok, true)

	fc.Spec.Join = true
	_, err = clientCluster.client.Resource("foreignclusters").Update(fc.Name, fc, metav1.UpdateOptions{})
	assert.NilError(t, err, "I can't set Join flag to true")

	// wait reconciliation
	time.Sleep(1 * time.Second)

	tmp, err = clientCluster.client.Resource("foreignclusters").Get("fc-test", metav1.GetOptions{})
	assert.NilError(t, err, "Error retrieving ForeignCluster")
	fc, ok = tmp.(*v1alpha1.ForeignCluster)
	assert.Equal(t, ok, true)
	assert.Equal(t, fc.Status.Outgoing.Joined, true, "Status OutgoingJoin is not true")
	assert.Assert(t, fc.Status.Outgoing.RemotePeeringRequestName != "", "Peering Request name can not be empty")
	assert.Assert(t, len(fc.Finalizers) > 0, "No finalizer has been set")
	assert.Assert(t, slice.ContainsString(fc.Finalizers, foreign_cluster_operator.FinalizerString, nil))

	tmp, err = clientCluster.client.Resource("peeringrequests").List(metav1.ListOptions{})
	assert.NilError(t, err)
	prs, ok := tmp.(*v1alpha1.PeeringRequestList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(prs.Items), 0, "Peering Request has been created on home cluster")

	tmp, err = serverCluster.client.Resource("peeringrequests").List(metav1.ListOptions{})
	assert.NilError(t, err)
	prs, ok = tmp.(*v1alpha1.PeeringRequestList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(prs.Items), 1, "Peering Request has not been created on foreign cluster")

	deploys, err := serverCluster.client.Client().AppsV1().Deployments("default").List(context.TODO(), metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Assert(t, len(deploys.Items) > 0, "Broadcaster deployment has not been created on foreign cluster")
	assert.Assert(t, func() bool {
		for _, deploy := range deploys.Items {
			if strings.Contains(deploy.Spec.Template.Spec.Containers[0].Image, "broadcaster") {
				return true
			}
		}
		return false
	}(), "No deployment found with broadcaster image")

	tmp, err = serverCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err)
	remoteFcList, ok := tmp.(*v1alpha1.ForeignClusterList)
	assert.Assert(t, ok)
	assert.Assert(t, len(remoteFcList.Items) == 1)
	assert.Assert(t, remoteFcList.Items[0].Spec.DiscoveryType == v1alpha1.IncomingPeeringDiscovery, "FC on remote cluster has not correct DiscoveryType")
	assert.Assert(t, remoteFcList.Items[0].Status.Incoming.PeeringRequest != nil, "PeeringRequest reference is not set")
	assert.Assert(t, remoteFcList.Items[0].Status.Incoming.Joined, "IncomingJoin flag not set on remote cluster")
	assert.Assert(t, remoteFcList.Items[0].Status.Incoming.AvailableIdentity, "AvailableIdentity not correctly set on remote cluster")
	assert.Assert(t, remoteFcList.Items[0].Status.Incoming.IdentityRef != nil, "IdentityRef not correctly set on remote cluster")

	// add local advertisement related to ForeignCluster,
	// we have to add it manually because we have no Advertisement Operator running in this test
	adv := &advtypes.Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name: "adv-test",
		},
		Spec: advtypes.AdvertisementSpec{
			LimitRange: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{},
			},
			Timestamp:  metav1.NewTime(time.Now()),
			TimeToLive: metav1.NewTime(time.Now()),
		},
	}
	tmp, err = clientCluster.advClient.Resource("advertisements").Create(adv, metav1.CreateOptions{})
	assert.NilError(t, err)
	adv, ok = tmp.(*advtypes.Advertisement)
	assert.Equal(t, ok, true)
	err = fc.SetAdvertisement(adv, clientCluster.client)
	assert.NilError(t, err)
}

// ------
// tests if disabling Join flag PeeringRequest is deleted from foreign cluster
func testUnjoin(t *testing.T) {
	tmp, err := clientCluster.client.Resource("foreignclusters").Get("fc-test", metav1.GetOptions{})
	assert.NilError(t, err, "Error retrieving ForeignCluster")
	fc, ok := tmp.(*v1alpha1.ForeignCluster)
	assert.Equal(t, ok, true)
	assert.Equal(t, fc.Spec.Join, true, "Foreign Cluster is not in join spec")
	assert.Equal(t, fc.Status.Outgoing.Joined, true, "Foreign Cluster is not joined")

	fc.Spec.Join = false
	_, err = clientCluster.client.Resource("foreignclusters").Update(fc.Name, fc, metav1.UpdateOptions{})
	assert.NilError(t, err, "I can't set Join flag to false")

	// wait reconciliation
	time.Sleep(1 * time.Second)

	tmp, err = clientCluster.client.Resource("foreignclusters").Get("fc-test", metav1.GetOptions{})
	assert.NilError(t, err, "Error retrieving ForeignCluster")
	fc, ok = tmp.(*v1alpha1.ForeignCluster)
	assert.Equal(t, ok, true)
	assert.Equal(t, fc.Status.Outgoing.Joined, false, "Status OutgoingJoin is true")
	assert.Assert(t, fc.Status.Outgoing.RemotePeeringRequestName == "", "Peering Request name has to be empty")
	assert.Assert(t, !slice.ContainsString(fc.Finalizers, foreign_cluster_operator.FinalizerString, nil), "Finalizer is not removed")

	tmp, err = serverCluster.client.Resource("peeringrequests").List(metav1.ListOptions{})
	assert.NilError(t, err, "Error listing PeeringRequests")
	prs, ok := tmp.(*v1alpha1.PeeringRequestList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(prs.Items), 0, "Peering Request has not been deleted on foreign cluster")

	time.Sleep(1 * time.Second)

	tmp, err = clientCluster.advClient.Resource("advertisements").List(metav1.ListOptions{})
	assert.NilError(t, err, "Error listing Advertisements")
	advs, ok := tmp.(*advtypes.AdvertisementList)
	assert.Equal(t, ok, true)
	assert.Equal(t, len(advs.Items), 1, "There is no Advertisement in local cluster")
	assert.Equal(t, advs.Items[0].Status.AdvertisementStatus, advtypes.AdvertisementDeleting, "The Advertisement is not in Deleting state")

	tmp, err = serverCluster.client.Resource("foreignclusters").List(metav1.ListOptions{})
	assert.NilError(t, err)
	remoteFcList, ok := tmp.(*v1alpha1.ForeignClusterList)
	assert.Assert(t, ok)
	assert.Assert(t, len(remoteFcList.Items) == 0, "ForeignCluster has not been deleted on remote cluster")
}

// ------
// tests bidirectional join
func testBidirectionalJoin(t *testing.T) {
	fc := &v1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "server-cluster",
		},
		Spec: v1alpha1.ForeignClusterSpec{
			ClusterIdentity: v1alpha1.ClusterIdentity{
				ClusterID: "server-cluster",
			},
			Namespace:     "default",
			Join:          true,
			ApiUrl:        serverCluster.cfg.Host,
			DiscoveryType: v1alpha1.ManualDiscovery,
		},
	}
	_, err := clientCluster.client.Resource("foreignclusters").Create(fc, metav1.CreateOptions{})
	assert.NilError(t, err, "Error during ForeignCluster creation")

	// wait reconciliation
	time.Sleep(1 * time.Second)

	tmp, err := serverCluster.client.Resource("foreignclusters").List(metav1.ListOptions{
		LabelSelector: "discovery-type=" + string(v1alpha1.IncomingPeeringDiscovery),
	})
	assert.NilError(t, err)
	remoteFcList, ok := tmp.(*v1alpha1.ForeignClusterList)
	assert.Assert(t, ok)
	assert.Assert(t, len(remoteFcList.Items) == 1)

	remoteFc := &remoteFcList.Items[0]
	remoteFc.Spec.Join = true
	_, err = serverCluster.client.Resource("foreignclusters").Update(remoteFc.Name, remoteFc, metav1.UpdateOptions{})
	assert.NilError(t, err)

	time.Sleep(1 * time.Second)

	tmp, err = serverCluster.client.Resource("foreignclusters").Get(remoteFc.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	remoteFc, ok = tmp.(*v1alpha1.ForeignCluster)
	assert.Assert(t, ok)
	assert.Assert(t, remoteFc.Status.Outgoing.Joined, "remote cluster has not joined us")

	tmp, err = clientCluster.client.Resource("peeringrequests").List(metav1.ListOptions{})
	assert.NilError(t, err)
	prList, ok := tmp.(*v1alpha1.PeeringRequestList)
	assert.Assert(t, ok)
	assert.Assert(t, len(prList.Items) == 1, "no incoming PeeringRequest")

	tmp, err = clientCluster.client.Resource("foreignclusters").Get("server-cluster", metav1.GetOptions{})
	assert.NilError(t, err)
	fc, ok = tmp.(*v1alpha1.ForeignCluster)
	assert.Assert(t, ok)
	assert.Assert(t, fc.Status.Incoming.PeeringRequest != nil, "PeeringRequest has not been set in local ForeignCluster")
	assert.Assert(t, fc.Status.Incoming.Joined && fc.Status.Outgoing.Joined, "Incoming and Outgoing flags ahs to be both set")
}

// ------
// tests clusters merge logic, when remote IP changes
func testMergeClusters(t *testing.T) {
	tmp, err := clientCluster.client.Resource("foreignclusters").Get("fc-test", metav1.GetOptions{})
	assert.NilError(t, err, "Error retrieving ForeignCluster")
	fc, ok := tmp.(*v1alpha1.ForeignCluster)
	assert.Equal(t, ok, true)

	txt := &discovery.TxtData{
		ID:        fc.Spec.ClusterIdentity.ClusterID,
		Namespace: fc.Spec.Namespace,
		ApiUrl:    strings.Replace(fc.Spec.ApiUrl, "127.0.0.1", "127.0.0.2", -1),
	}
	fc, err = clientCluster.discoveryCtrl.CheckUpdate(txt, fc, fc.Spec.DiscoveryType)
	assert.NilError(t, err)
	assert.Equal(t, fc.Spec.ApiUrl, txt.ApiUrl, "API URL not changed")

	time.Sleep(100 * time.Millisecond)

	tmp, err = clientCluster.client.Resource("foreignclusters").Get("fc-test", metav1.GetOptions{})
	assert.NilError(t, err, "Error retrieving ForeignCluster")
	fc, ok = tmp.(*v1alpha1.ForeignCluster)
	assert.Equal(t, ok, true)

	cfg, err := fc.GetConfig(clientCluster.client.Client())
	assert.NilError(t, err, "Unable to load foreign CA Data")
	assert.Assert(t, cfg != nil, "Unable to load foreign config")
}

// ------
// tests kubeconfig creation
func testCreateKubeconfig(t *testing.T) {
	// setup
	token := base64.StdEncoding.EncodeToString([]byte("token"))
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sa-secret",
		},
		Data: map[string][]byte{
			"token": []byte(token),
		},
	}
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sa",
		},
		Secrets: []corev1.ObjectReference{
			{
				Name: secret.Name,
			},
		},
	}
	_, err := clientCluster.client.Client().CoreV1().Secrets("default").Create(context.TODO(), secret, metav1.CreateOptions{})
	assert.NilError(t, err)
	_, err = clientCluster.client.Client().CoreV1().ServiceAccounts("default").Create(context.TODO(), sa, metav1.CreateOptions{})
	assert.NilError(t, err)

	err = os.Setenv("APISERVER", "127.0.0.2")
	assert.NilError(t, err)

	// test
	kc, err := kubeconfig.CreateKubeConfig(clientCluster.client.Client(), sa.Name, "default")
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(kc, "127.0.0.2"), "API server ip not set")
	assert.Assert(t, strings.Contains(kc, "6443"), "default port not set")

	err = os.Setenv("APISERVER_PORT", "1234")
	assert.NilError(t, err)

	kc, err = kubeconfig.CreateKubeConfig(clientCluster.client.Client(), sa.Name, "default")
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(kc, "1234"), "non-default port not set")
}
