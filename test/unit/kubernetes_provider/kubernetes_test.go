package kubernetes_provider

import (
	provider "github.com/netgroup-polito/dronev2/internal/kubernetes"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)


func TestH2FCreation(t *testing.T) {
	pHome := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "toto", Namespace: "test", UID: "0973c9af-35aa-4050-929d-bc8bc3fc3b5a",
		},
		Spec:   v1.PodSpec{
			NodeName: "trololo",
		},
		Status: v1.PodStatus{},
	}
	pForeign := provider.H2FTranslate(pHome, "")
	assert.Empty(t, pForeign.UID, "The UID of translated pod should be null")
	assert.Empty(t, pForeign.Spec.NodeName, "The node name should not be set")
	assert.Equal(t, string(pHome.UID), pForeign.GetAnnotations()["home_uuid"])
	assert.Equal(t, pHome.CreationTimestamp.String(), pForeign.GetAnnotations()["home_creationTimestamp"])
	assert.Equal(t, pHome.Spec.NodeName, pForeign.GetAnnotations()["home_nodename"])
	assert.Equal(t, pHome.ResourceVersion, pForeign.GetAnnotations()["home_resourceVersion"])

}

func TestF2HCreation(t *testing.T) {
	annotations := make(map[string]string)
	annotations["home_nodename"]= "toto"
	annotations["home_resourceVersion"]= "508"
	annotations["home_uuid"]= "42131279-7e1a-427e-b521-042326145c59"
	annotations["home_creationTimestamp"]= "2020-01-15T13:21:18Z"
	ForeignObjectMeta := metav1.ObjectMeta{
		Name:                       "test",
		Namespace:                  "test",
		Annotations: annotations,
	}

	podIPs := make([]v1.PodIP, 1)
	podIP := v1.PodIP{IP:"10.16.1.2"}
	podIPs =append(podIPs,podIP)
	pForeign := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: ForeignObjectMeta,
		Spec:   v1.PodSpec{},
		Status: v1.PodStatus{
			PodIP: "10.16.1.2",
			PodIPs: podIPs,
	 },
	}

	newForeignPodCidr := "172.42.0.0/16"
	expectedPodIP := "172.42.1.2"

	pHome := provider.F2HTranslate(pForeign, newForeignPodCidr, "")
	assert.Equal(t, pHome.UID, types.UID(pForeign.GetAnnotations()["home_uuid"]))
	assert.Equal(t, pHome.Status.PodIP, expectedPodIP)
}


