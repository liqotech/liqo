package crdReplicator

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/consts"
)

const (
	testNamespace = "default"
)

var (
	tunGVR = schema.GroupVersionResource{
		Group:    netv1alpha1.GroupVersion.Group,
		Version:  netv1alpha1.GroupVersion.Version,
		Resource: "tunnelendpoints",
	}
	fcGVR = schema.GroupVersionResource{
		Group:    "discovery.liqo.io",
		Version:  "v1alpha1",
		Resource: "foreignclusters",
	}
)

func setupDispatcherOperator() error {
	var err error
	localDynClient := dynamic.NewForConfigOrDie(k8sManagerLocal.GetConfig())
	localDynFac := dynamicinformer.NewFilteredDynamicSharedInformerFactory(localDynClient, crdreplicator.ResyncPeriod, metav1.NamespaceAll, crdreplicator.SetLabelsForLocalResources)
	dOperator = &crdreplicator.Controller{
		Scheme:                         k8sManagerLocal.GetScheme(),
		Client:                         k8sManagerLocal.GetClient(),
		ClientSet:                      nil,
		ClusterID:                      localClusterID,
		RemoteDynClients:               peeringClustersDynClients, // we already populate the dynamicClients of the peering clusters
		LocalDynClient:                 localDynClient,
		LocalDynSharedInformerFactory:  localDynFac,
		RemoteDynSharedInformerFactory: peeringClustersDynFactories,
		RegisteredResources:            nil,
		UnregisteredResources:          nil,
		LocalWatchers:                  make(map[string]chan struct{}),
		RemoteWatchers:                 make(map[string]map[string]chan struct{}),
	}
	err = dOperator.SetupWithManager(k8sManagerLocal)
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}
	return nil
}

func getTunnelEndpointResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "net.liqo.io/v1alpha1",
			"kind":       "TunnelEndpoint",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": testNamespace,
				"labels":    map[string]string{},
			},
			"spec": map[string]interface{}{
				"clusterID":      "clusterid-test",
				"podCIDR":        "10.0.0.0/12",
				"externalCIDR":   "172.16.0.0/16",
				"endpointIP":     "192.16.5.1",
				"backendType":    "wireguard",
				"backend_config": map[string]interface{}{},
			},
		},
	}
}

func getForeignClusterResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "discovery.liqo.io/v1alpha1",
			"kind":       "ForeignCluster",
			"metadata": map[string]interface{}{
				"name":   "test",
				"labels": map[string]string{},
			},
			"spec": map[string]interface{}{
				"clusterIdentity": map[string]interface{}{
					"clusterID": "foreign-cluster",
				},
				"join":           true,
				"foreignAuthUrl": "https://192.168.2.100:30001",
			},
		},
	}
}

func cleanUp(t *testing.T, localResources map[string]*netv1alpha1.TunnelEndpoint) {
	for _, res := range localResources {
		err := dOperator.LocalDynClient.Resource(tunGVR).Namespace(testNamespace).Delete(context.TODO(), res.Name, metav1.DeleteOptions{})
		klog.Infof("deleting resource %s", res.Name)
		assert.Nil(t, err, "should be nil")
		time.Sleep(10 * time.Second)
	}
	// check that the resources have been removed from the peering clusters
	for clusterID, dynClient := range peeringClustersDynClients {
		_, err := dynClient.Resource(tunGVR).Namespace(testNamespace).Get(context.TODO(), localResources[clusterID].Name, metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(err), "error should be not found")
	}
}

// the dynamicClients to the peering clusters are created from the foreignCluster
// while testing we already have those dynamicClients so the foreignCluster resource
// is used only to trigger the reconcile logic

// we create a resource which type has been registered for the replication
// but we don't label it, so we expect to not find it on the remote clusters
func TestReplication1(t *testing.T) {
	time.Sleep(10 * time.Second)
	// first we create a tunnelEndpoint on the localCluster
	tun := getTunnelEndpointResource()
	newTun, err := dOperator.LocalDynClient.Resource(tunGVR).Namespace(testNamespace).Create(context.TODO(), tun, metav1.CreateOptions{})
	assert.Nil(t, err, "error should be nil")

	time.Sleep(2 * time.Second)
	// check that the resource does not exist on the remote clusters
	for _, dynClient := range peeringClustersDynClients {
		_, err := dynClient.Resource(tunGVR).Namespace(testNamespace).Get(context.TODO(), tun.GetName(), metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(err), "error should be not found")
	}
	// delete resources
	err = dOperator.LocalDynClient.Resource(tunGVR).Namespace(testNamespace).Delete(context.TODO(), newTun.GetName(), metav1.DeleteOptions{})
	assert.Nil(t, err, "error should be nil")

}

// we create a resource which type has been registered for the replication
// we label it to be replicated on all the three clusters, so we expect to not find it on the remote clusters
func TestReplication2(t *testing.T) {
	time.Sleep(10 * time.Second)
	localResources := map[string]*netv1alpha1.TunnelEndpoint{}
	// we create the resource on the localcluster to be replicated on all the peeringClusters
	for clusterID := range peeringClustersTestEnvs {
		tun := getTunnelEndpointResource()
		tun.SetName(clusterID)
		tun.SetLabels(map[string]string{
			crdreplicator.DestinationLabel:   clusterID,
			crdreplicator.LocalLabelSelector: "true",
		})
		newTun, err := dOperator.LocalDynClient.Resource(tunGVR).Namespace(testNamespace).Create(context.TODO(), tun, metav1.CreateOptions{})
		assert.Nil(t, err, "error should be nil")
		typedTun := &netv1alpha1.TunnelEndpoint{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(newTun.Object, typedTun)
		assert.Nil(t, err, "error should be nil")
		localResources[clusterID] = typedTun
	}

	time.Sleep(10 * time.Second)
	// check that the replication happened on the peering clusters and that the spec is the same.
	for clusterID, dynClient := range peeringClustersDynClients {
		typedTun := &netv1alpha1.TunnelEndpoint{}
		remTun, err := dynClient.Resource(tunGVR).Namespace(testNamespace).Get(context.TODO(), localResources[clusterID].Name, metav1.GetOptions{})
		assert.Nil(t, err, "error should be nil")
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(remTun.Object, typedTun)
		assert.Nil(t, err, "error should be nil")
		// check that the local and the replicated one are the same
		assert.True(t, reflect.DeepEqual(typedTun.Spec, localResources[clusterID].Spec))
	}
	// here we remove all the resources on the local cluster and check that also the remote ones have been removed
	cleanUp(t, localResources)
	time.Sleep(3 * time.Second)
}

// we create a resource which type has been registered for the replication
// we label it to be replicated on all the three clusters, so we expect to find it on the remote clusters
// we update the status on the peering clusters and expect it to be replicated on the local cluster as well
func TestReplication4(t *testing.T) {
	updateOwnership(consts.OwnershipShared)
	time.Sleep(10 * time.Second)
	localResources := map[string]*netv1alpha1.TunnelEndpoint{}
	// we create the resource on the localcluster to be replicated on all the peeringClusters
	for clusterID := range peeringClustersTestEnvs {
		tun := getTunnelEndpointResource()
		tun.SetName(clusterID)
		tun.SetLabels(map[string]string{
			crdreplicator.DestinationLabel:   clusterID,
			crdreplicator.LocalLabelSelector: "true",
		})
		newTun, err := dOperator.LocalDynClient.Resource(tunGVR).Namespace(testNamespace).Create(context.TODO(), tun, metav1.CreateOptions{})
		assert.Nil(t, err, "error should be nil")
		typedTun := &netv1alpha1.TunnelEndpoint{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(newTun.Object, typedTun)
		assert.Nil(t, err, "error should be nil")
		localResources[clusterID] = typedTun
	}

	time.Sleep(10 * time.Second)
	// check that the resources have been replicated on the peering clusters
	for clusterID, dynClient := range peeringClustersDynClients {
		remTun, err := dynClient.Resource(tunGVR).Namespace(testNamespace).Get(context.TODO(), localResources[clusterID].Name, metav1.GetOptions{})
		assert.Nil(t, err, "error should be nil")
		typedTun := &netv1alpha1.TunnelEndpoint{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(remTun.Object, typedTun)
		assert.Nil(t, err, "error should be nil")
		// check that the local and the replicated one are the same
		assert.True(t, reflect.DeepEqual(typedTun.Spec, localResources[clusterID].Spec))
	}

	// here we update the status of the remote instances
	for clusterID, tun := range localResources {
		status := map[string]interface{}{
			"phase": "Ready",
		}
		currentTun, err := peeringClustersDynClients[clusterID].Resource(tunGVR).
			Namespace(testNamespace).Get(context.TODO(), tun.Name, metav1.GetOptions{})
		assert.Nil(t, err, "error should be nil")
		err = unstructured.SetNestedMap(currentTun.Object, status, "status")
		assert.Nil(t, err, "error should be nil")
		_, err = peeringClustersDynClients[clusterID].Resource(tunGVR).
			Namespace(testNamespace).UpdateStatus(context.TODO(), currentTun, metav1.UpdateOptions{})
		assert.Nil(t, err, "error should be nil")
		time.Sleep(10 * time.Second)
	}

	// retrieve the local resources from the local cluster and check if the update has been replicated
	for _, tun := range localResources {
		remTun, err := dOperator.LocalDynClient.Resource(tunGVR).
			Namespace(testNamespace).Get(context.TODO(), tun.GetName(), metav1.GetOptions{})
		assert.Nil(t, err, "error should be nil")
		typedTun := &netv1alpha1.TunnelEndpoint{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(remTun.Object, typedTun)
		assert.Nil(t, err, "error should be nil")
		// check that the local and the replicated one are the same
		assert.Equal(t, "Ready", typedTun.Status.Phase, "phase on remote resources should be set to 'Ready'")
	}
	// here we remove all the resources on the local cluster and check that also the remote ones have been removed
	cleanUp(t, localResources)

	// err = dOperator.LocalDynClient.Resource(fcGVR).Delete(context.TODO(), newFc.GetName(), metav1.DeleteOptions{})
	time.Sleep(3 * time.Second)
	updateOwnership(consts.OwnershipLocal)
}

// we create a resource which type has been registered for the replication
// we label it to be replicated on all the three clusters, so we expect to not find it on the remote clusters
// we update the status and expect it to be replicated on the peering clusters as well
func TestReplication3(t *testing.T) {
	time.Sleep(10 * time.Second)
	localResources := map[string]*netv1alpha1.TunnelEndpoint{}
	// we create the resource on the localcluster to be replicated on all the peeringClusters
	for clusterID := range peeringClustersTestEnvs {
		tun := getTunnelEndpointResource()
		tun.SetName(clusterID)
		tun.SetLabels(map[string]string{
			crdreplicator.DestinationLabel:   clusterID,
			crdreplicator.LocalLabelSelector: "true",
		})
		newTun, err := dOperator.LocalDynClient.Resource(tunGVR).Namespace(testNamespace).Create(context.TODO(), tun, metav1.CreateOptions{})
		assert.Nil(t, err, "error should be nil")
		typedTun := &netv1alpha1.TunnelEndpoint{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(newTun.Object, typedTun)
		assert.Nil(t, err, "error should be nil")
		localResources[clusterID] = typedTun
	}
	time.Sleep(10 * time.Second)

	// check that the resources have been replicated on the peering clusters
	for clusterID, dynClient := range peeringClustersDynClients {
		remTun, err := dynClient.Resource(tunGVR).Namespace(testNamespace).Get(context.TODO(), localResources[clusterID].Name, metav1.GetOptions{})
		assert.Nil(t, err, "error should be nil")
		typedTun := &netv1alpha1.TunnelEndpoint{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(remTun.Object, typedTun)
		assert.Nil(t, err, "error should be nil")
		// check that the local and the replicated one are the same
		assert.True(t, reflect.DeepEqual(typedTun.Spec, localResources[clusterID].Spec))
	}

	// here we update the status of the local instances
	for _, tun := range localResources {
		status := map[string]interface{}{
			"phase": "Ready",
		}
		currentTun, err := dOperator.LocalDynClient.Resource(tunGVR).Namespace(testNamespace).Get(context.TODO(), tun.Name, metav1.GetOptions{})
		assert.Nil(t, err, "error should be nil")
		err = unstructured.SetNestedMap(currentTun.Object, status, "status")
		assert.Nil(t, err, "error should be nil")
		_, err = dOperator.LocalDynClient.Resource(tunGVR).Namespace(testNamespace).UpdateStatus(context.TODO(), currentTun, metav1.UpdateOptions{})
		assert.Nil(t, err, "error should be nil")
		time.Sleep(10 * time.Second)
	}

	// retrieve the replicated resources from the peering cluster and check if the update is present
	for clusterID, dynClient := range peeringClustersDynClients {
		remTun, err := dynClient.Resource(tunGVR).Namespace(testNamespace).Get(context.TODO(), localResources[clusterID].Name, metav1.GetOptions{})
		assert.Nil(t, err, "error should be nil")
		typedTun := &netv1alpha1.TunnelEndpoint{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(remTun.Object, typedTun)
		assert.Nil(t, err, "error should be nil")
		// check that the local and the replicated one are the same
		assert.True(t, reflect.DeepEqual(typedTun.Spec, localResources[clusterID].Spec))
		assert.Equal(t, "Ready", typedTun.Status.Phase, "phase on remote resources should be set to 'Ready'")
	}
	// here we remove all the resources on the local cluster and check that also the remote ones have been removed
	cleanUp(t, localResources)
}
