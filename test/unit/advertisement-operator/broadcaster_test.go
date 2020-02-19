package advertisement_operator

import (
	"fmt"
	"github.com/netgroup-polito/dronev2/internal/advertisement-operator"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"
)

func createFakeResources() ([]v1.Node, []v1.ContainerImage, resource.Quantity) {
	nodes := make([]v1.Node, 10)
	images := make([]v1.ContainerImage, 10)

	sum := resource.Quantity{}

	for i := 0; i < len(nodes); i++ {
		resources := v1.ResourceList{}
		q := *resource.NewQuantity(int64(i), resource.DecimalSI)
		resources[v1.ResourceCPU] = q
		resources[v1.ResourceMemory] = q
		resources[v1.ResourcePods] = q

		sum.Add(q)

		im := make([]v1.ContainerImage, 1)
		im[0].Names = append(im[0].Names, fmt.Sprint(i))
		im[0].SizeBytes = int64(i)
		images[i] = im[0]

		nodes[i] = v1.Node{
			Status: v1.NodeStatus{
				Allocatable: resources,
				Images:   im,
			},
		}
	}

	return nodes, images, sum
}

func TestGetClusterResources(t *testing.T) {

	nodes, images, sum := createFakeResources()
	res, im := advertisement_operator.GetClusterResources(nodes)

	assert.Empty(t, res.StorageEphemeral(), "StorageEphemeral was not set so it should be null")
	assert.Equal(t, *res.Cpu(), sum)
	assert.Equal(t, *res.Memory(), sum)
	assert.Equal(t, *res.Pods(), sum)
	assert.Equal(t, im, images)
}

func TestComputePrices(t *testing.T) {
	_, images, _ := createFakeResources()
	prices := advertisement_operator.ComputePrices(images)

	keys1 := make([]string, len(prices))
	keys2 := make([]string, len(prices))

	for key, _ := range prices {
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

	nodes, images, _ := createFakeResources()
	availability, _ := advertisement_operator.GetClusterResources(nodes)
	adv := advertisement_operator.CreateAdvertisement(nodes, "fake-cluster")

	assert.NotEmpty(t, adv.Name, "Name should be provided")
	assert.NotEmpty(t, adv.Namespace, "Namespace should be set")
	assert.Empty(t, adv.ResourceVersion)
	assert.NotEmpty(t, adv.Spec.ClusterId)
	assert.NotEmpty(t, adv.Spec.Timestamp)
	assert.NotEmpty(t, adv.Spec.Validity)
	assert.Equal(t, adv.Name, "advertisement-fake-cluster")
	assert.Equal(t, images, adv.Spec.Images)
	assert.Equal(t, availability, adv.Spec.Availability)
	assert.Empty(t, adv.Status, "Status should not be set")
}
