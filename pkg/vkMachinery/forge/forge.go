package forge

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	vk "github.com/liqotech/liqo/pkg/vkMachinery"
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
								Key:      liqoconst.TypeNode,
								Operator: v1.NodeSelectorOpNotIn,
								Values:   []string{liqoconst.TypeNode},
							},
						},
					},
				},
			},
		},
	}
}

func forgeVKVolumes() []v1.Volume {
	volumes := []v1.Volume{
		{
			Name: vk.VKCertsVolumeName,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
	}
	return volumes
}

func forgeVKInitContainers(nodeName, initVKImage string) []v1.Container {
	return []v1.Container{
		{
			Resources: forgeVKResources(),
			Name:      "crt-generator",
			Image:     initVKImage,
			Command: []string{
				"/usr/bin/init-virtual-kubelet",
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
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      vk.VKCertsVolumeName,
					MountPath: vk.VKCertsRootPath,
				},
			},
		},
	}
}

func forgeVKContainers(
	vkImage string, remoteClusterID,
	nodeName, vkNamespace, liqoNamespace, homeClusterID string) []v1.Container {
	command := []string{
		"/usr/bin/virtual-kubelet",
	}

	args := []string{
		stringifyArgument("--foreign-cluster-id", remoteClusterID),
		stringifyArgument("--provider", "kubernetes"),
		stringifyArgument("--nodename", nodeName),
		stringifyArgument("--kubelet-namespace", vkNamespace),
		stringifyArgument("--home-cluster-id", homeClusterID),
		stringifyArgument("--ipam-server", fmt.Sprintf("%v.%v", liqoconst.NetworkManagerServiceName, liqoNamespace)),
		"--enable-node-lease",
		"--klog.v=4",
	}

	volumeMounts := []v1.VolumeMount{
		{
			Name:      vk.VKCertsVolumeName,
			MountPath: vk.VKCertsRootPath,
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
					Value: vk.CertLocation,
				},
				{
					Name:  "APISERVER_KEY_LOCATION",
					Value: vk.KeyLocation,
				},
				{
					Name:      "VKUBELET_POD_IP",
					ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "status.podIP", APIVersion: "v1"}},
				},
				{
					Name:  "VKUBELET_TAINT_KEY",
					Value: liqoconst.VirtualNodeTolerationKey,
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

func forgeVKPodSpec(
	vkName, vkNamespace, liqoNamespace, homeClusterID string,
	remoteClusterID, initVKImage, nodeName, vkImage string) v1.PodSpec {
	return v1.PodSpec{
		Volumes:        forgeVKVolumes(),
		InitContainers: forgeVKInitContainers(nodeName, initVKImage),
		Containers: forgeVKContainers(vkImage, remoteClusterID,
			nodeName, vkNamespace, liqoNamespace, homeClusterID),
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

func stringifyArgument(key, value string) string {
	return fmt.Sprintf("%s=%s", key, value)
}
