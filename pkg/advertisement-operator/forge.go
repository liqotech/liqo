package advertisementOperator

import (
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	liqoControllerManager "github.com/liqotech/liqo/pkg/liqo-controller-manager"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const vkCPUResourceReq = "300m"
const vkMemoryResourceReq = "100M"
const vkCPUResourceLim = "1000m"
const vkMemoryResourceLim = "250M"

func forgeVKAffinity() *v1.Affinity {
	return &v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      liqoControllerManager.TypeNode,
								Operator: v1.NodeSelectorOpNotIn,
								Values:   []string{liqoControllerManager.TypeNode},
							},
						},
					},
				},
			},
		},
	}
}

func forgeVKVolumes(adv *advtypes.Advertisement) []v1.Volume {
	return []v1.Volume{
		{
			Name: "remote-kubeconfig",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: adv.Spec.KubeConfigRef.Name,
				},
			},
		},
		{
			Name: "virtual-kubelet-crt",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
	}
}

func forgeVKInitContainers(nodeName string, initVKImage string) []v1.Container {
	return []v1.Container{
		{
			Resources: forgeVKResources(),
			Name:      "crt-generator",
			Image:     initVKImage,
			Command: []string{
				"/usr/bin/local/kubelet-setup.sh",
			},
			Env: []v1.EnvVar{
				{
					Name:      "POD_IP",
					ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "status.podIP", APIVersion: "v1"}},
				},
				{
					Name:      "POD_NAME",
					ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.name", APIVersion: "v1"}},
				},
				{
					Name:  "NODE_NAME",
					Value: nodeName,
				},
			},
			Args: []string{
				"/etc/virtual-kubelet/certs",
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "virtual-kubelet-crt",
					MountPath: "/etc/virtual-kubelet/certs",
				},
			},
		},
	}
}

func forgeVKContainers(vkImage string, adv *advtypes.Advertisement, nodeName string, vkNamespace string, homeClusterId string) []v1.Container {
	command := []string{
		"/usr/bin/virtual-kubelet",
	}

	args := []string{
		"--foreign-cluster-id",
		adv.Spec.ClusterId,
		"--provider",
		"kubernetes",
		"--nodename",
		nodeName,
		"--kubelet-namespace",
		vkNamespace,
		"--foreign-kubeconfig",
		"/app/kubeconfig/remote",
		"--home-cluster-id",
		homeClusterId,
	}

	volumeMounts := []v1.VolumeMount{
		{
			Name:      "remote-kubeconfig",
			MountPath: "/app/kubeconfig/remote",
			SubPath:   "kubeconfig",
		},
		{
			Name:      "virtual-kubelet-crt",
			MountPath: "/etc/virtual-kubelet/certs",
		},
	}

	return []v1.Container{
		{
			Name:         "virtual-kubelet",
			Resources:    forgeVKResources(),
			Image:        vkImage,
			Command:      command,
			Args:         args,
			VolumeMounts: volumeMounts,
			Env: []v1.EnvVar{
				{
					Name:  "APISERVER_CERT_LOCATION",
					Value: "/etc/virtual-kubelet/certs/server.crt",
				},
				{
					Name:  "APISERVER_KEY_LOCATION",
					Value: "/etc/virtual-kubelet/certs/server-key.pem",
				},
				{
					Name:      "VKUBELET_POD_IP",
					ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "status.podIP", APIVersion: "v1"}},
				},
				{
					Name:  "VKUBELET_TAINT_KEY",
					Value: "virtual-node.liqo.io/not-allowed",
				},
				{
					Name:  "VKUBELET_TAINT_VALUE",
					Value: "true",
				},
				{
					Name:  "VKUBELET_TAINT_EFFECT",
					Value: "NoExecute",
				},
			},
		},
	}
}

func forgeVKPodSpec(vkName string, vkNamespace string, homeClusterId string, adv *advtypes.Advertisement, initVKImage string, nodeName string, vkImage string) v1.PodSpec {
	return v1.PodSpec{
		Volumes:            forgeVKVolumes(adv),
		InitContainers:     forgeVKInitContainers(nodeName, initVKImage),
		Containers:         forgeVKContainers(vkImage, adv, nodeName, vkNamespace, homeClusterId),
		ServiceAccountName: vkName,
		Affinity:           forgeVKAffinity(),
	}
}

func forgeVKResources() v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Limits: v1.ResourceList{
			"cpu":    resource.MustParse(vkCPUResourceLim),
			"memory": resource.MustParse(vkMemoryResourceLim),
		},
		Requests: v1.ResourceList{
			"cpu":    resource.MustParse(vkCPUResourceReq),
			"memory": resource.MustParse(vkMemoryResourceReq),
		},
	}
}
