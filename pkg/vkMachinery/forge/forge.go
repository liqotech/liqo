// Copyright 2019-2023 The Liqo Authors
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
	"github.com/liqotech/liqo/pkg/utils/pod"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	vk "github.com/liqotech/liqo/pkg/vkMachinery"
)

func getDefaultStorageClass(storageClasses []sharingv1alpha1.StorageType) sharingv1alpha1.StorageType {
	for _, storageClass := range storageClasses {
		if storageClass.Default {
			return storageClass
		}
	}
	return storageClasses[0]
}

func forgeVKContainers(
	vkImage string, homeCluster, remoteCluster *discoveryv1alpha1.ClusterIdentity,
	nodeName, vkNamespace, liqoNamespace string, opts *VirtualKubeletOpts,
	resourceOffer *sharingv1alpha1.ResourceOffer) []v1.Container {
	command := []string{
		"/usr/bin/virtual-kubelet",
	}

	args := []string{
		stringifyArgument("--foreign-cluster-id", remoteCluster.ClusterID),
		stringifyArgument("--foreign-cluster-name", remoteCluster.ClusterName),
		stringifyArgument("--nodename", nodeName),
		stringifyArgument("--node-ip", "$(POD_IP)"),
		stringifyArgument("--tenant-namespace", vkNamespace),
		stringifyArgument("--home-cluster-id", homeCluster.ClusterID),
		stringifyArgument("--home-cluster-name", homeCluster.ClusterName),
		stringifyArgument("--ipam-server",
			fmt.Sprintf("%v.%v:%v", liqoconst.NetworkManagerServiceName, liqoNamespace, liqoconst.NetworkManagerIpamPort)),
	}

	if len(resourceOffer.Spec.StorageClasses) > 0 {
		args = append(args, "--enable-storage",
			stringifyArgument("--remote-real-storage-class-name",
				getDefaultStorageClass(resourceOffer.Spec.StorageClasses).StorageClassName))
	}

	if extraAnnotations := opts.NodeExtraAnnotations.StringMap; len(extraAnnotations) != 0 {
		args = append(args, stringifyArgument("--node-extra-annotations", opts.NodeExtraAnnotations.String()))
	}
	if extraLabels := opts.NodeExtraLabels.StringMap; len(extraLabels) != 0 {
		args = append(args, stringifyArgument("--node-extra-labels", opts.NodeExtraLabels.String()))
	}

	args = append(args, opts.ExtraArgs...)

	return []v1.Container{
		{
			Name:      "virtual-kubelet",
			Resources: pod.ForgeContainerResources(opts.RequestsCPU, opts.LimitsCPU, opts.RequestsRAM, opts.LimitsRAM),
			Image:     vkImage,
			Command:   command,
			Args:      args,
			Env: []v1.EnvVar{
				{
					Name:      "POD_IP",
					ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "status.podIP"}},
				},
			},
		},
	}
}

func forgeVKPodSpec(
	vkNamespace, liqoNamespace string,
	homeCluster, remoteCluster *discoveryv1alpha1.ClusterIdentity, opts *VirtualKubeletOpts,
	resourceOffer *sharingv1alpha1.ResourceOffer) v1.PodSpec {
	nodeName := virtualKubelet.VirtualNodeName(remoteCluster)
	return v1.PodSpec{
		Containers: forgeVKContainers(opts.ContainerImage, homeCluster, remoteCluster,
			nodeName, vkNamespace, liqoNamespace, opts, resourceOffer),
		ServiceAccountName: vk.ServiceAccountName,
	}
}

func stringifyArgument(key, value string) string {
	return fmt.Sprintf("%s=%s", key, value)
}
