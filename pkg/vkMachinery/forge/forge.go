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
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/pod"
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
	nodeName, vkNamespace string, storageClasses []sharingv1alpha1.StorageType, opts *VirtualKubeletOpts) []v1.Container {
	command := []string{
		"/usr/bin/virtual-kubelet",
	}

	args := []string{
		stringifyArgument(string(ForeignClusterID), remoteCluster.ClusterID),
		stringifyArgument(string(ForeignClusterName), remoteCluster.ClusterName),
		stringifyArgument(string(NodeName), nodeName),
		stringifyArgument(string(NodeIP), "$(POD_IP)"),
		stringifyArgument(string(TenantNamespace), vkNamespace),
		stringifyArgument(string(HomeClusterID), homeCluster.ClusterID),
		stringifyArgument(string(HomeClusterName), homeCluster.ClusterName),
	}

	if opts.IpamEndpoint != "" {
		args = append(args, stringifyArgument(string(IpamEndpoint), opts.IpamEndpoint))
	}

	if len(storageClasses) > 0 {
		args = append(args, string(EnableStorage),
			stringifyArgument(string(RemoteRealStorageClassName),
				getDefaultStorageClass(storageClasses).StorageClassName))
	}

	args = appendArgsReflectorsWorkers(args, opts.ReflectorsWorkers)
	args = appendArgsReflectorsType(args, opts.ReflectorsType)

	if len(opts.LabelsNotReflected) > 0 {
		args = append(args, stringifyArgument(string(LabelsNotReflected), strings.Join(opts.LabelsNotReflected, ",")))
	}

	if len(opts.AnnotationsNotReflected) > 0 {
		args = append(args, stringifyArgument(string(AnnotationsNotReflected), strings.Join(opts.AnnotationsNotReflected, ",")))
	}

	if extraAnnotations := opts.NodeExtraAnnotations.StringMap; len(extraAnnotations) != 0 {
		args = append(args, stringifyArgument(string(NodeExtraAnnotations), opts.NodeExtraAnnotations.String()))
	}
	if extraLabels := opts.NodeExtraLabels.StringMap; len(extraLabels) != 0 {
		args = append(args, stringifyArgument(string(NodeExtraLabels), opts.NodeExtraLabels.String()))
	}

	args = append(args, opts.ExtraArgs...)

	containerPorts := []v1.ContainerPort{}
	args = append(args, stringifyArgument(string(MetricsEnabled), strconv.FormatBool(opts.MetricsEnabled)))
	if opts.MetricsEnabled {
		args = append(args, stringifyArgument(string(MetricsAddress), opts.MetricsAddress))
		metrAddrSplit := strings.Split(opts.MetricsAddress, ":")
		metricsPort, err := strconv.ParseInt(metrAddrSplit[len(metrAddrSplit)-1], 10, 32)
		if err != nil {
			metrAddrSplit := strings.Split(vk.MetricsAddress, ":")
			// if the metrics address is not valid, use the default one
			metricsPort, _ = strconv.ParseInt(metrAddrSplit[len(metrAddrSplit)-1], 10, 32)
		}
		containerPorts = append(containerPorts, v1.ContainerPort{
			Name:          "metrics",
			ContainerPort: int32(metricsPort),
			Protocol:      v1.ProtocolTCP,
		})
	}

	return []v1.Container{
		{
			Name:      vk.ContainerName,
			Resources: pod.ForgeContainerResources(opts.RequestsCPU, opts.LimitsCPU, opts.RequestsRAM, opts.LimitsRAM),
			Image:     vkImage,
			Command:   command,
			Args:      args,
			Env: []v1.EnvVar{
				{
					Name:      "POD_IP",
					ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "status.podIP"}},
				},
				{
					Name:      "POD_NAME",
					ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.name"}},
				},
			},
			Ports: containerPorts,
		},
	}
}

func forgeVKPodSpec(
	vkNamespace string,
	homeCluster *discoveryv1alpha1.ClusterIdentity, virtualNode *virtualkubeletv1alpha1.VirtualNode, opts *VirtualKubeletOpts) v1.PodSpec {
	return v1.PodSpec{
		Containers: forgeVKContainers(opts.ContainerImage, homeCluster, virtualNode.Spec.ClusterIdentity,
			virtualNode.Name, vkNamespace, virtualNode.Spec.StorageClasses, opts),
		ServiceAccountName: virtualNode.Name,
	}
}

func stringifyArgument(key, value string) string {
	return fmt.Sprintf("%s=%s", key, value)
}

func appendArgsReflectorsWorkers(args []string, reflectorsWorkers map[string]*uint) []string {
	for resource, value := range reflectorsWorkers {
		key := fmt.Sprintf("--%s-reflection-workers", resource)
		args = append(args, stringifyArgument(key, strconv.Itoa(int(*value))))
	}
	return args
}

func appendArgsReflectorsType(args []string, reflectorsType map[string]*string) []string {
	for resource, value := range reflectorsType {
		key := fmt.Sprintf("--%s-reflection-type", resource)
		args = append(args, stringifyArgument(key, *value))
	}
	return args
}
