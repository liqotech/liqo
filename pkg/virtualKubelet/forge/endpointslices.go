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
	corev1 "k8s.io/api/core/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	discoveryv1beta1apply "k8s.io/client-go/applyconfigurations/discovery/v1beta1"
)

// EndpointSliceManagedBy -> The manager associated with the reflected EndpointSlices.
const EndpointSliceManagedBy = "endpointslice.reflection.liqo.io"

// EndpointTranslator defines the function to translate between local and remote endpoint addresses.
type EndpointTranslator func([]string) []string

// EndpointSliceLabels returns the labels assigned to the reflected EndpointSlices.
func EndpointSliceLabels() labels.Set {
	return map[string]string{discoveryv1beta1.LabelManagedBy: EndpointSliceManagedBy}
}

// IsEndpointSliceManagedByReflection returns whether the EndpointSlice is managed by the reflection logic.
func IsEndpointSliceManagedByReflection(obj metav1.Object) bool {
	return EndpointSliceLabels().AsSelectorPreValidated().Matches(labels.Set(obj.GetLabels()))
}

// EndpointToBeReflected filters out the endpoints targeting pods already running on the remote cluster.
func EndpointToBeReflected(endpoint *discoveryv1beta1.Endpoint) bool {
	// NodeName needs to be enabled through a feature gate in the v1beta1 API.
	if endpoint.NodeName != nil {
		return *endpoint.NodeName != LiqoNodeName
	}

	// The topology field is deprecated and will be removed when the v1beta1 API is removed (no sooner than kubernetes v1.24).
	return endpoint.Topology[corev1.LabelHostname] != LiqoNodeName
}

// RemoteEndpointSlice forges the apply patch for the reflected endpointslice, given the local one.
func RemoteEndpointSlice(local *discoveryv1beta1.EndpointSlice, targetNamespace string,
	translator EndpointTranslator) *discoveryv1beta1apply.EndpointSliceApplyConfiguration {
	return discoveryv1beta1apply.EndpointSlice(local.GetName(), targetNamespace).
		WithLabels(local.GetLabels()).WithLabels(ReflectionLabels()).
		WithLabels(EndpointSliceLabels()).WithAnnotations(local.GetAnnotations()).
		WithAddressType(local.AddressType).
		WithEndpoints(RemoteEndpointSliceEndpoints(local.Endpoints, translator)...).
		WithPorts(RemoteEndpointSlicePorts(local.Ports)...)
}

// RemoteEndpointSliceEndpoints forges the apply patch for the endpoints of the reflected endpointslice, given the local ones.
func RemoteEndpointSliceEndpoints(locals []discoveryv1beta1.Endpoint,
	translator EndpointTranslator) []*discoveryv1beta1apply.EndpointApplyConfiguration {
	var remotes []*discoveryv1beta1apply.EndpointApplyConfiguration
	hostname := LocalClusterID

	for i := range locals {
		if !EndpointToBeReflected(&locals[i]) {
			// Skip the endpoints referring to the target node (as natively present).
			continue
		}

		local := locals[i].DeepCopy()
		conditions := &discoveryv1beta1apply.EndpointConditionsApplyConfiguration{Ready: local.Conditions.Ready}

		remote := discoveryv1beta1apply.Endpoint().
			WithAddresses(translator(local.Addresses)...).WithConditions(conditions).
			WithTopology(local.Topology).WithTopology(map[string]string{corev1.LabelHostname: hostname}).
			WithTargetRef(RemoteObjectReference(local.TargetRef))
		remote.Hostname = local.Hostname

		remotes = append(remotes, remote)
	}

	return remotes
}

// RemoteEndpointSlicePorts forges the apply patch for the ports of the reflected endpointslice, given the local ones.
func RemoteEndpointSlicePorts(locals []discoveryv1beta1.EndpointPort) []*discoveryv1beta1apply.EndpointPortApplyConfiguration {
	var remotes []*discoveryv1beta1apply.EndpointPortApplyConfiguration

	for i := range locals {
		// DeepCopy the local object, to avoid mutating the cache.
		local := locals[i].DeepCopy()
		remotes = append(remotes, &discoveryv1beta1apply.EndpointPortApplyConfiguration{
			Name: local.Name, Port: local.Port, Protocol: local.Protocol, AppProtocol: local.AppProtocol,
		})
	}

	return remotes
}
