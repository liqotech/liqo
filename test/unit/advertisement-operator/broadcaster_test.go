package advertisement_operator

import (
	"context"
	"fmt"
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/advertisement-operator"
	"github.com/liqoTech/liqo/internal/kubernetes/test"
	pkg "github.com/liqoTech/liqo/pkg/advertisement-operator"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"testing"
	"time"
)

func createBroadcaster(clusterConfig policyv1.ClusterConfigSpec) advertisement_operator.AdvertisementBroadcaster {
	// set the client in fake mode
	crdClient.Fake = true

	// create fake client for the home cluster
	homeClient, err := protocolv1.CreateAdvertisementClient("", nil)
	if err != nil {
		panic(err)
	}

	// create the fake client for the foreign cluster
	foreignClient, err := protocolv1.CreateAdvertisementClient("", nil)
	if err != nil {
		panic(err)
	}

	// create the discovery client
	discoveryClient, err := discoveryv1.CreatePeeringRequestClient("")
	if err != nil {
		panic(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Data:       nil,
		StringData: nil,
		Type:       "",
	}

	return advertisement_operator.AdvertisementBroadcaster{
		LocalClient:                homeClient,
		DiscoveryClient:            discoveryClient,
		KubeconfigSecretForForeign: secret,
		RemoteClient:               foreignClient,
		HomeClusterId:              test.HomeClusterId,
		ForeignClusterId:           test.ForeignClusterId,
		GatewayPrivateIP:           "10.0.0.1",
		ClusterConfig:              clusterConfig,
		PeeringRequestName:         test.ForeignClusterId,
	}
}

func createFakeResources() (physicalNodes *corev1.NodeList, virtualNodes *corev1.NodeList, images []corev1.ContainerImage, sum resource.Quantity, podList *corev1.PodList) {
	pNodes := make([]corev1.Node, 5)
	vNodes := make([]corev1.Node, 5)
	physicalNodes = new(corev1.NodeList)
	virtualNodes = new(corev1.NodeList)
	images = make([]corev1.ContainerImage, 5)
	sum = resource.Quantity{}

	p := 0
	v := 0

	for i := 0; i < 10; i++ {
		resources := corev1.ResourceList{}
		q := *resource.NewQuantity(int64(i), resource.DecimalSI)
		resources[corev1.ResourceCPU] = q
		resources[corev1.ResourceMemory] = q
		resources[corev1.ResourcePods] = q

		if i%2 == 0 {
			// physical node
			im := make([]corev1.ContainerImage, 1)
			im[0].Names = append(im[0].Names, fmt.Sprint(p))
			im[0].SizeBytes = int64(p)
			images[p] = im[0]

			pNodes[p] = corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "pnode-" + strconv.Itoa(p),
					Labels: make(map[string]string),
				},
				Spec: corev1.NodeSpec{
					PodCIDR: fmt.Sprintf("%d.%d.%d.%d/%d", i, i, i, i, 16),
				},
				Status: corev1.NodeStatus{
					Allocatable: resources,
					Images:      im,
					Addresses:   []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: fmt.Sprintf("%d.%d.%d.%d", i, i, i, i)}},
				},
			}
			sum.Add(q)
			p++
		} else {
			// virtual node
			vNodes[v] = corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "liqo-cluster" + strconv.Itoa(v),
					Labels: map[string]string{"type": "virtual-node"},
				},
				Spec: corev1.NodeSpec{
					PodCIDR: fmt.Sprintf("%d.%d.%d.%d/%d", i, i, i, i, 16),
				},
				Status: corev1.NodeStatus{
					Allocatable: resources,
					Addresses:   []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: fmt.Sprintf("%d.%d.%d.%d", i, i, i, i)}},
				},
			}
			v++
		}
	}

	p, v = 0, 0
	pods := make([]corev1.Pod, 10)
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			// pods on physical nodes
			pods[i] = corev1.Pod{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-" + strconv.Itoa(i),
				},
				Spec: corev1.PodSpec{
					NodeName: pNodes[p].Name,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			}
			p++
		} else {
			// pods on virtual nodes
			pods[i] = corev1.Pod{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod-" + strconv.Itoa(i),
				},
				Spec: corev1.PodSpec{
					NodeName: vNodes[v].Name,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			}
			v++
		}
	}
	pNodes[0].Labels["liqonet.liqo.io/gateway"] = "true"
	physicalNodes.Items = pNodes
	virtualNodes.Items = vNodes
	podList = &corev1.PodList{Items: pods}

	return physicalNodes, virtualNodes, images, sum, podList
}

func createResourcesOnCluster(client *crdClient.CRDClient, pNodes *corev1.NodeList, vNodes *corev1.NodeList, pods *corev1.PodList) error {
	// create resources on home cluster
	for i := 0; i < len(pNodes.Items); i++ {
		_, err := client.Client().CoreV1().Nodes().Create(context.TODO(), &pNodes.Items[i], metav1.CreateOptions{})
		if err != nil {
			return err
		}
		_, err = client.Client().CoreV1().Nodes().Create(context.TODO(), &vNodes.Items[i], metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	for i := 0; i < len(pods.Items); i++ {
		_, err := client.Client().CoreV1().Pods("").Create(context.TODO(), &pods.Items[i], metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func prepareAdv(b advertisement_operator.AdvertisementBroadcaster) protocolv1.Advertisement {
	pNodes, vNodes, images, _, pods := createFakeResources()
	reqs, limits := advertisement_operator.GetAllPodsResources(pods)
	availability, _ := advertisement_operator.ComputeAnnouncedResources(pNodes, reqs, int64(b.ClusterConfig.AdvertisementConfig.OutgoingConfig.ResourceSharingPercentage))
	neighbours := make(map[corev1.ResourceName]corev1.ResourceList)
	for _, vNode := range vNodes.Items {
		neighbours[corev1.ResourceName(vNode.Name)] = vNode.Status.Allocatable
	}
	return b.CreateAdvertisement(pNodes, vNodes, availability, images, limits)
}

func TestGetClusterResources(t *testing.T) {

	pNodes, _, images, sum, _ := createFakeResources()
	res, im := advertisement_operator.GetClusterResources(pNodes.Items)

	assert.Empty(t, res.StorageEphemeral(), "StorageEphemeral was not set so it should be null")
	assert.Equal(t, *res.Cpu(), sum)
	assert.Equal(t, *res.Memory(), sum)
	assert.Equal(t, *res.Pods(), sum)
	assert.Equal(t, im, images)
}

func TestComputePrices(t *testing.T) {
	_, _, images, _, _ := createFakeResources()
	prices := advertisement_operator.ComputePrices(images)

	keys1 := make([]string, len(prices))
	keys2 := make([]string, len(prices))

	for key := range prices {
		keys1 = append(keys1, key.String())
	}
	for _, im := range images {
		keys2 = append(keys2, im.Names[0])
	}
	keys2 = append(keys2, "cpu")
	keys2 = append(keys2, "memory")

	assert.ElementsMatch(t, keys1, keys2)
}

func TestCreateAdvertisement(t *testing.T) {
	pNodes, vNodes, images, _, pods := createFakeResources()
	gatewayNode := pNodes.Items[0]
	sharingPercentage := int32(50)
	reqs, limits := advertisement_operator.GetAllPodsResources(pods)
	availability, _ := advertisement_operator.ComputeAnnouncedResources(pNodes, reqs, int64(sharingPercentage))
	neighbours := make(map[corev1.ResourceName]corev1.ResourceList)
	for _, vNode := range vNodes.Items {
		neighbours[corev1.ResourceName(vNode.Name)] = vNode.Status.Allocatable
	}

	clusterConfig := createFakeClusterConfig()
	broadcaster := createBroadcaster(clusterConfig.Spec)

	adv := broadcaster.CreateAdvertisement(pNodes, vNodes, availability, images, limits)

	assert.NotEmpty(t, adv.Name, "Name should be provided")
	assert.Empty(t, adv.ResourceVersion)
	assert.Equal(t, broadcaster.HomeClusterId, adv.Spec.ClusterId)
	assert.NotEmpty(t, adv.Spec.KubeConfigRef)
	assert.NotEmpty(t, adv.Spec.Timestamp)
	assert.NotEmpty(t, adv.Spec.TimeToLive)
	assert.Equal(t, "advertisement-"+broadcaster.HomeClusterId, adv.Name)
	assert.Equal(t, images, adv.Spec.Images)
	assert.Equal(t, availability, adv.Spec.ResourceQuota.Hard)
	assert.Equal(t, limits, adv.Spec.LimitRange.Limits[0].Max)
	assert.Equal(t, neighbours, adv.Spec.Neighbors)
	assert.Equal(t, pNodes.Items[0].Spec.PodCIDR, adv.Spec.Network.PodCIDR)
	assert.Equal(t, gatewayNode.Status.Addresses[0].Address, adv.Spec.Network.GatewayIP)
	assert.Equal(t, "10.0.0.1", adv.Spec.Network.GatewayPrivateIP)
	assert.Empty(t, adv.Status, "Status should not be set")
}

func TestGetResourceForAdv(t *testing.T) {
	clusterConfig := createFakeClusterConfig()
	b := createBroadcaster(clusterConfig.Spec)
	pNodes, vNodes, images, _, pods := createFakeResources()

	err := createResourcesOnCluster(b.LocalClient, pNodes, vNodes, pods)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)

	reqs, limits := advertisement_operator.GetAllPodsResources(pods)
	availability, _ := advertisement_operator.ComputeAnnouncedResources(pNodes, reqs, int64(b.ClusterConfig.AdvertisementConfig.OutgoingConfig.ResourceSharingPercentage))
	if availability.Cpu().Value() < 0 || availability.Memory().Value() < 0 {
		t.Fatal("Available resources cannot be negative")
	}

	pNodes2, vNodes2, availability2, limits2, images2, err := b.GetResourcesForAdv()
	assert.Nil(t, err)
	assert.Equal(t, pNodes, pNodes2)
	assert.Equal(t, vNodes, vNodes2)
	assert.Equal(t, availability, availability2)
	assert.Equal(t, limits, limits2)
	assert.Equal(t, images, images2)
}

func TestSendAdvertisementCreation(t *testing.T) {
	clusterConfig := createFakeClusterConfig()
	b := createBroadcaster(clusterConfig.Spec)
	_, err := b.RemoteClient.Client().CoreV1().Secrets(b.KubeconfigSecretForForeign.Namespace).Create(context.TODO(), b.KubeconfigSecretForForeign, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// create adv on foreign cluster
	adv := prepareAdv(b)
	adv2, err := b.SendAdvertisementToForeignCluster(adv)
	assert.Nil(t, err)
	assert.Equal(t, b.KubeconfigSecretForForeign.OwnerReferences, pkg.GetOwnerReference(adv2))

	// update adv on foreign cluster
	adv.Spec.ResourceQuota.Hard[corev1.ResourceCPU] = resource.MustParse("10")
	adv3, err := b.SendAdvertisementToForeignCluster(adv)
	assert.Nil(t, err)
	assert.Equal(t, adv.Spec.ResourceQuota.Hard.Cpu().Value(), adv3.Spec.ResourceQuota.Hard.Cpu().Value())
}

func TestNotifyAdvertisementDeletion(t *testing.T) {
	clusterConfig := createFakeClusterConfig()
	b := createBroadcaster(clusterConfig.Spec)
	_, err := b.RemoteClient.Client().CoreV1().Secrets(b.KubeconfigSecretForForeign.Namespace).Create(context.TODO(), b.KubeconfigSecretForForeign, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// create adv on foreign cluster
	adv := prepareAdv(b)
	adv2, _ := b.SendAdvertisementToForeignCluster(adv)
	// modify adv status to DELETING
	err = b.NotifyAdvertisementDeletion()
	time.Sleep(1 * time.Second)
	assert.Nil(t, err)
	assert.Equal(t, advertisement_operator.AdvertisementDeleting, adv2.Status.AdvertisementStatus)
}
