package kubernetes_provider

import (
	"github.com/liqotech/liqo/pkg/virtualKubelet/translation"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func createFakeVolumesAndVolumeMounts() ([]v1.Volume, []v1.VolumeMount) {
	return []v1.Volume{
			{
				Name: "secret-test",
				VolumeSource: v1.VolumeSource{
					Secret: &v1.SecretVolumeSource{},
				},
			},
			{
				Name: "configMap-test",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{},
				},
			},
			{
				Name: "emptyDir-test",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "downwardAPI-test",
				VolumeSource: v1.VolumeSource{
					DownwardAPI: &v1.DownwardAPIVolumeSource{},
				},
			},
			{
				Name: "unmanaged-test",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{},
				},
			},
			{
				Name: "default-token-test",
				VolumeSource: v1.VolumeSource{
					Secret: &v1.SecretVolumeSource{
						SecretName: "default-token-12345",
					},
				},
			},
		},
		[]v1.VolumeMount{
			{
				Name: "secret-test",
			},
			{
				Name: "configMap-test",
			},
			{
				Name: "emptyDir-test",
			},
			{
				Name: "downwardAPI-test",
			},
			{
				Name: "unmanaged-test",
			},
			{
				Name: "default-token-test",
			},
		}
}

func createFakeContainers(volumeMounts []v1.VolumeMount) []v1.Container {
	return []v1.Container{
		{
			Name:            "test",
			Image:           "test",
			Command:         nil,
			Args:            nil,
			WorkingDir:      "",
			Ports:           nil,
			Env:             nil,
			Resources:       v1.ResourceRequirements{},
			VolumeMounts:    volumeMounts,
			LivenessProbe:   nil,
			ReadinessProbe:  nil,
			StartupProbe:    nil,
			SecurityContext: nil,
		},
		{
			Name:         "test2",
			Image:        "test2",
			VolumeMounts: []v1.VolumeMount{},
		},
	}
}

func TestH2FCreation(t *testing.T) {

	volumes, volumeMounts := createFakeVolumesAndVolumeMounts()
	filteredVolumes := translation.FilterVolumes(volumes)
	filteredVolumeMounts := translation.FilterVolumeMounts(filteredVolumes, volumeMounts)
	containers := createFakeContainers(filteredVolumeMounts)

	pHome := &v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "toto", Namespace: "test", UID: "0973c9af-35aa-4050-929d-bc8bc3fc3b5a",
		},
		Spec: v1.PodSpec{
			NodeName:   "trololo",
			Containers: containers,
			Volumes:    volumes,
		},
		Status: v1.PodStatus{},
	}
	pForeign := translation.H2FTranslate(pHome, "")

	assert.Empty(t, pForeign.UID, "The UID of translated pod should be null")
	assert.Empty(t, pForeign.Spec.NodeName, "The NodeName should not be set")
	assert.NotEmpty(t, pForeign.Spec.Affinity, "The Affinity should be set")
	assert.NotEmpty(t, pForeign.Spec.Containers, "The Containers should be set")
	assert.NotEmpty(t, pForeign.Spec.Volumes, "The Volumes should be set")
	assert.Equal(t, string(pHome.UID), pForeign.GetAnnotations()["home_uuid"])
	assert.Equal(t, pHome.CreationTimestamp.String(), pForeign.GetAnnotations()["home_creationTimestamp"])
	assert.Equal(t, pHome.Spec.NodeName, pForeign.GetAnnotations()["home_nodename"])
	assert.Equal(t, pHome.ResourceVersion, pForeign.GetAnnotations()["home_resourceVersion"])
	assert.ElementsMatch(t, containers, pForeign.Spec.Containers)
	assert.ElementsMatch(t, filteredVolumeMounts, pForeign.Spec.Containers[0].VolumeMounts)
	assert.ElementsMatch(t, filteredVolumes, pForeign.Spec.Volumes)
}

func TestF2HCreation(t *testing.T) {
	annotations := make(map[string]string)
	annotations["home_nodename"] = "toto"
	annotations["home_resourceVersion"] = "508"
	annotations["home_uuid"] = "42131279-7e1a-427e-b521-042326145c59"
	annotations["home_creationTimestamp"] = "2020-01-15T13:21:18Z"
	ForeignObjectMeta := metav1.ObjectMeta{
		Name:        "test",
		Namespace:   "test",
		Annotations: annotations,
	}

	podIPs := make([]v1.PodIP, 1)
	podIP := v1.PodIP{IP: "10.16.1.2"}
	podIPs = append(podIPs, podIP)
	pForeign := &v1.Pod{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: ForeignObjectMeta,
		Spec:       v1.PodSpec{},
		Status: v1.PodStatus{
			PodIP:  "10.16.1.2",
			PodIPs: podIPs,
		},
	}

	newForeignPodCidr := "172.42.0.0/16"
	expectedPodIP := "172.42.1.2"

	pHome := translation.F2HTranslate(pForeign, newForeignPodCidr, "")
	assert.Equal(t, pHome.UID, types.UID(pForeign.GetAnnotations()["home_uuid"]))
	assert.Equal(t, pHome.Status.PodIP, expectedPodIP)
}

func TestFilterVolumes(t *testing.T) {
	// create a list of 6 volumes:
	// the first 4 should be copied
	// the fifth one is of an unmanaged type and should be filtered
	// the sixth one is a default-token secret and should be filtered
	volumes, _ := createFakeVolumesAndVolumeMounts()
	expectedResult := []v1.Volume{
		{
			Name: "secret-test",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{},
			},
		},
		{
			Name: "configMap-test",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{},
			},
		},
		{
			Name: "emptyDir-test",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "downwardAPI-test",
			VolumeSource: v1.VolumeSource{
				DownwardAPI: &v1.DownwardAPIVolumeSource{},
			},
		},
	}
	result := translation.FilterVolumes(volumes)

	assert.ElementsMatch(t, expectedResult, result)
}

func TestFilterVolumeMounts(t *testing.T) {
	volumes, volumeMounts := createFakeVolumesAndVolumeMounts()
	filteredVolumes := translation.FilterVolumes(volumes)
	expectedResult := []v1.VolumeMount{
		{
			Name: "secret-test",
		},
		{
			Name: "configMap-test",
		},
		{
			Name: "emptyDir-test",
		},
		{
			Name: "downwardAPI-test",
		},
	}
	result := translation.FilterVolumeMounts(filteredVolumes, volumeMounts)

	assert.ElementsMatch(t, expectedResult, result)
}
