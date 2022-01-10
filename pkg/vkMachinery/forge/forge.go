// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package forge

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	vk "github.com/liqotech/liqo/pkg/vkMachinery"
)

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

func forgeVKInitContainers(nodeName string, opts *VirtualKubeletOpts) []v1.Container {
	if opts.DisableCertGeneration {
		return []v1.Container{}
	}

	return []v1.Container{
		{
			Resources: forgeVKResources(opts),
			Name:      "crt-generator",
			Image:     opts.InitContainerImage,
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
					Name:      "POD_NAMESPACE",
					ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.namespace", APIVersion: "v1"}},
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

func getDeafultStorageClass(storageClasses []sharingv1alpha1.StorageType) sharingv1alpha1.StorageType {
	for _, storageClass := range storageClasses {
		if storageClass.Default {
			return storageClass
		}
	}
	return storageClasses[0]
}

func forgeVKContainers(
	vkImage string, homeCluster, remoteCluster discoveryv1alpha1.ClusterIdentity,
	nodeName, vkNamespace, liqoNamespace string, opts *VirtualKubeletOpts,
	resourceOffer *sharingv1alpha1.ResourceOffer) []v1.Container {
	command := []string{
		"/usr/bin/virtual-kubelet",
	}

	args := []string{
		stringifyArgument("--foreign-cluster-id", remoteCluster.ClusterID),
		stringifyArgument("--foreign-cluster-name", remoteCluster.ClusterName),
		stringifyArgument("--nodename", nodeName),
		stringifyArgument("--tenant-namespace", vkNamespace),
		stringifyArgument("--home-cluster-id", homeCluster.ClusterID),
		stringifyArgument("--home-cluster-name", homeCluster.ClusterName),
		stringifyArgument("--ipam-server",
			fmt.Sprintf("%v.%v:%v", liqoconst.NetworkManagerServiceName, liqoNamespace, liqoconst.NetworkManagerIpamPort)),
	}

	if len(resourceOffer.Spec.StorageClasses) > 0 {
		args = append(args, "--enable-storage",
			stringifyArgument("--remote-real-storage-class-name",
				getDeafultStorageClass(resourceOffer.Spec.StorageClasses).StorageClassName))
	}

	if extraAnnotations := opts.NodeExtraAnnotations.StringMap; len(extraAnnotations) != 0 {
		args = append(args, stringifyArgument("--node-extra-annotations", opts.NodeExtraAnnotations.String()))
	}
	if extraLabels := opts.NodeExtraLabels.StringMap; len(extraLabels) != 0 {
		args = append(args, stringifyArgument("--node-extra-labels", opts.NodeExtraLabels.String()))
	}
	args = append(args, opts.ExtraArgs...)

	volumeMounts := []v1.VolumeMount{
		{
			Name:      vk.VKCertsVolumeName,
			MountPath: vk.VKCertsRootPath,
		},
	}

	return []v1.Container{
		{
			Name:         "virtual-kubelet",
			Resources:    forgeVKResources(opts),
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
			},
		},
	}
}

func forgeVKPodSpec(
	vkName, vkNamespace, liqoNamespace string,
	homeCluster, remoteCluster discoveryv1alpha1.ClusterIdentity, nodeName string, opts *VirtualKubeletOpts,
	resourceOffer *sharingv1alpha1.ResourceOffer) v1.PodSpec {
	return v1.PodSpec{
		Volumes:        forgeVKVolumes(),
		InitContainers: forgeVKInitContainers(nodeName, opts),
		Containers: forgeVKContainers(opts.ContainerImage, homeCluster, remoteCluster,
			nodeName, vkNamespace, liqoNamespace, opts, resourceOffer),
		ServiceAccountName: vkName,
		Affinity:           forgeVKAffinity(),
	}
}

func forgeVKResources(opts *VirtualKubeletOpts) v1.ResourceRequirements {
	return v1.ResourceRequirements{
		Limits: v1.ResourceList{
			v1.ResourceCPU:    opts.LimitsCPU,
			v1.ResourceMemory: opts.LimitsRAM,
		},
		Requests: v1.ResourceList{
			v1.ResourceCPU:    opts.RequestsCPU,
			v1.ResourceMemory: opts.RequestsRAM,
		},
	}
}

func stringifyArgument(key, value string) string {
	return fmt.Sprintf("%s=%s", key, value)
}
