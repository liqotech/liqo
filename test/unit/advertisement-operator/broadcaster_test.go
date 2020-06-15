package advertisement_operator

import (
	"fmt"
	"github.com/liqoTech/liqo/internal/advertisement-operator"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"testing"
)

func createFakeResources() (physicalNodes *v1.NodeList, virtualNodes *v1.NodeList, images []v1.ContainerImage, sum resource.Quantity, podList *v1.PodList) {
	pNodes := make([]v1.Node, 5)
	vNodes := make([]v1.Node, 5)
	physicalNodes = new(v1.NodeList)
	virtualNodes = new(v1.NodeList)
	images = make([]v1.ContainerImage, 5)
	sum = resource.Quantity{}

	p := 0
	v := 0

	for i := 0; i < 10; i++ {
		resources := v1.ResourceList{}
		q := *resource.NewQuantity(int64(i), resource.DecimalSI)
		resources[v1.ResourceCPU] = q
		resources[v1.ResourceMemory] = q
		resources[v1.ResourcePods] = q

		if i%2 == 0 {
			im := make([]v1.ContainerImage, 1)
			im[0].Names = append(im[0].Names, fmt.Sprint(p))
			im[0].SizeBytes = int64(p)
			images[p] = im[0]

			pNodes[p] = v1.Node{
				Spec: v1.NodeSpec{
					PodCIDR: fmt.Sprintf("%d.%d.%d.%d/%d", i, i, i, i, 16),
				},
				Status: v1.NodeStatus{
					Allocatable: resources,
					Images:      im,
					Addresses:   []v1.NodeAddress{{Type: v1.NodeExternalIP, Address: fmt.Sprintf("%d.%d.%d.%d", i, i, i, i)}},
				},
			}
			sum.Add(q)
			p++
		} else {
			vNodes[v] = v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"type": "virtual-node"},
				},
				Spec: v1.NodeSpec{
					PodCIDR: fmt.Sprintf("%d.%d.%d.%d/%d", i, i, i, i, 16),
				},
				Status: v1.NodeStatus{
					Allocatable: resources,
					Addresses:   []v1.NodeAddress{{Type: v1.NodeExternalIP, Address: fmt.Sprintf("%d.%d.%d.%d", i, i, i, i)}},
				},
			}
			v++
		}
	}

	pods := make([]v1.Pod, 10)
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			pods[i] = v1.Pod{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1.PodSpec{
					NodeName: "vk-cluster" + strconv.Itoa(i),
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
				},
			}
		} else {
			pods[i] = v1.Pod{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1.PodSpec{
					NodeName: "node" + strconv.Itoa(i),
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
				},
			}
		}
	}
	podList = &v1.PodList{Items: pods}
	physicalNodes.Items = pNodes
	virtualNodes.Items = vNodes
	return physicalNodes, virtualNodes, images, sum, podList
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
	reqs, limits := advertisement_operator.GetAllPodsResources(pods)
	availability, _ := advertisement_operator.ComputeAnnouncedResources(pNodes, reqs)
	neighbours := make(map[v1.ResourceName]v1.ResourceList)
	for _, vNode := range vNodes.Items {
		neighbours[v1.ResourceName(vNode.Name)] = vNode.Status.Allocatable
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Data:       nil,
		StringData: nil,
		Type:       "",
	}

	broadcaster := advertisement_operator.AdvertisementBroadcaster{
		KubeconfigSecretForForeign: secret,
		HomeClusterId:              "fake-cluster",
		GatewayIP:                  "1.2.3.4",
		GatewayPrivateIP:           "10.0.0.1",
	}

	adv := broadcaster.CreateAdvertisement(pNodes, vNodes, availability, images, limits)

	assert.NotEmpty(t, adv.Name, "Name should be provided")
	assert.NotEmpty(t, adv.Namespace, "Namespace should be set")
	assert.Empty(t, adv.ResourceVersion)
	assert.NotEmpty(t, adv.Spec.ClusterId)
	assert.NotEmpty(t, adv.Spec.KubeConfigRef)
	assert.NotEmpty(t, adv.Spec.Timestamp)
	assert.NotEmpty(t, adv.Spec.TimeToLive)
	assert.Equal(t, adv.Name, "advertisement-fake-cluster")
	assert.Equal(t, images, adv.Spec.Images)
	assert.Equal(t, availability, adv.Spec.ResourceQuota.Hard)
	assert.Equal(t, limits, adv.Spec.LimitRange.Limits[0].Max)
	assert.Equal(t, neighbours, adv.Spec.Neighbors)
	assert.Equal(t, pNodes.Items[0].Spec.PodCIDR, adv.Spec.Network.PodCIDR)
	assert.Equal(t, "1.2.3.4", adv.Spec.Network.GatewayIP)
	assert.Equal(t, "10.0.0.1", adv.Spec.Network.GatewayPrivateIP)
	assert.Empty(t, adv.Status, "Status should not be set")
}
