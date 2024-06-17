// Copyright 2019-2024 The Liqo Authors
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

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
	vk "github.com/liqotech/liqo/pkg/vkMachinery"
)

// StringifyArgument returns a string representation of the key-value pair.
func StringifyArgument(key, value string) string {
	return fmt.Sprintf("%s=%s", key, value)
}

// DestringifyArgument returns the key and the value of the string representation of the key-value pair.
func DestringifyArgument(arg string) (key, value string) {
	split := strings.SplitN(arg, "=", 2)
	return split[0], split[1]
}

func getDefaultStorageClass(storageClasses []authv1alpha1.StorageType) authv1alpha1.StorageType {
	for _, storageClass := range storageClasses {
		if storageClass.Default {
			return storageClass
		}
	}
	return storageClasses[0]
}

func getDefaultIngressClass(ingressClasses []authv1alpha1.IngressType) authv1alpha1.IngressType {
	for _, ingressClass := range ingressClasses {
		if ingressClass.Default {
			return ingressClass
		}
	}
	return ingressClasses[0]
}

func getDefaultLoadBalancerClass(loadBalancerClasses []authv1alpha1.LoadBalancerType) authv1alpha1.LoadBalancerType {
	for _, loadBalancerClass := range loadBalancerClasses {
		if loadBalancerClass.Default {
			return loadBalancerClass
		}
	}
	return loadBalancerClasses[0]
}

func forgeVKContainers(
	homeCluster, remoteCluster discoveryv1alpha1.ClusterID,
	nodeName, vkNamespace, localPodCIDR, liqoNamespace string,
	storageClasses []authv1alpha1.StorageType, ingressClasses []authv1alpha1.IngressType, loadBalancerClasses []authv1alpha1.LoadBalancerType,
	opts *vkv1alpha1.VkOptionsTemplate) []v1.Container {
	command := []string{
		"/usr/bin/virtual-kubelet",
	}

	args := []string{
		StringifyArgument(string(ForeignClusterID), string(remoteCluster)),
		StringifyArgument(string(NodeName), nodeName),
		StringifyArgument(string(NodeIP), "$(POD_IP)"),
		StringifyArgument(string(TenantNamespace), vkNamespace),
		StringifyArgument(string(LiqoNamespace), liqoNamespace),
		StringifyArgument(string(HomeClusterID), string(homeCluster)),
		StringifyArgument(string(LocalPodCIDR), localPodCIDR),
	}

	if len(storageClasses) > 0 {
		args = append(args, string(EnableStorage),
			StringifyArgument(string(RemoteRealStorageClassName),
				getDefaultStorageClass(storageClasses).StorageClassName))
	}
	if len(ingressClasses) > 0 {
		args = append(args, string(EnableIngress),
			StringifyArgument(string(RemoteRealIngressClassName),
				getDefaultIngressClass(ingressClasses).IngressClassName))
	}
	if len(loadBalancerClasses) > 0 {
		args = append(args, string(EnableLoadBalancer),
			StringifyArgument(string(RemoteRealLoadBalancerClassName),
				getDefaultLoadBalancerClass(loadBalancerClasses).LoadBalancerClassName))
	}

	args = appendArgsReflectorsWorkers(args, opts.Spec.ReflectorsConfig)
	args = appendArgsReflectorsType(args, opts.Spec.ReflectorsConfig)

	if extraAnnotations := opts.Spec.NodeExtraAnnotations; len(extraAnnotations) != 0 {
		stringifiedMap := argsutils.StringMap{StringMap: extraAnnotations}.String()
		args = append(args, StringifyArgument(string(NodeExtraAnnotations), stringifiedMap))
	}
	if extraLabels := opts.Spec.NodeExtraLabels; len(extraLabels) != 0 {
		stringifiedMap := argsutils.StringMap{StringMap: extraLabels}.String()
		args = append(args, StringifyArgument(string(NodeExtraLabels), stringifiedMap))
	}

	args = append(args, opts.Spec.ExtraArgs...)

	containerPorts := []v1.ContainerPort{}
	args = append(args, StringifyArgument(string(MetricsEnabled), strconv.FormatBool(opts.Spec.MetricsEnabled)))
	if opts.Spec.MetricsEnabled {
		args = append(args, StringifyArgument(string(MetricsAddress), opts.Spec.MetricsAddress))
		metrAddrSplit := strings.Split(opts.Spec.MetricsAddress, ":")
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
			Resources: opts.Spec.Resources,
			Image:     opts.Spec.ContainerImage,
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
				{
					Name:      "POD_NAMESPACE",
					ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.namespace"}},
				},
				{
					Name:  "VIRTUALNODE_NAME",
					Value: nodeName,
				},
			},
			Ports: containerPorts,
		},
	}
}

func forgeVKPodSpec(vkNamespace string, homeCluster discoveryv1alpha1.ClusterID, localPodCIDR, liqoNamespace string,
	virtualNode *vkv1alpha1.VirtualNode, opts *vkv1alpha1.VkOptionsTemplate) v1.PodSpec {
	return v1.PodSpec{
		Containers: forgeVKContainers(
			homeCluster, virtualNode.Spec.ClusterID,
			virtualNode.Name, vkNamespace, localPodCIDR, liqoNamespace,
			virtualNode.Spec.StorageClasses, virtualNode.Spec.IngressClasses, virtualNode.Spec.LoadBalancerClasses,
			opts),
		ServiceAccountName: virtualNode.Name,
	}
}

func appendArgsReflectorsWorkers(args []string, reflectorsConfig map[string]vkv1alpha1.ReflectorConfig) []string {
	if reflectorsConfig == nil {
		return args
	}

	for _, resource := range resources.Reflectors {
		reflector, ok := reflectorsConfig[string(resource)]
		if !ok {
			continue
		}
		key := fmt.Sprintf("--%s-reflection-workers", resource)
		args = append(args, StringifyArgument(key, strconv.Itoa(int(reflector.NumWorkers))))
	}

	return args
}

func appendArgsReflectorsType(args []string, reflectorsConfig map[string]vkv1alpha1.ReflectorConfig) []string {
	if reflectorsConfig == nil {
		return args
	}

	for _, resource := range resources.ReflectorsCustomizableType {
		reflector, ok := reflectorsConfig[string(resource)]
		if !ok {
			continue
		}
		key := fmt.Sprintf("--%s-reflection-type", resource)
		args = append(args, StringifyArgument(key, string(reflector.Type)))
	}

	return args
}
