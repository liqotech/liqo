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
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	discoveryv1apply "k8s.io/client-go/applyconfigurations/discovery/v1"
	"k8s.io/utils/pointer"
)

// EndpointSliceManagedBy -> The manager associated with the reflected EndpointSlices.
const EndpointSliceManagedBy = "endpointslice.reflection.liqo.io"

// EndpointTranslator defines the function to translate between local and remote endpoint addresses.
type EndpointTranslator func([]string) []string

// EndpointSliceLabels returns the labels assigned to the reflected EndpointSlices.
func EndpointSliceLabels() labels.Set {
	return map[string]string{discoveryv1.LabelManagedBy: EndpointSliceManagedBy}
}

// IsEndpointSliceManagedByReflection returns whether the EndpointSlice is managed by the reflection logic.
func IsEndpointSliceManagedByReflection(obj metav1.Object) bool {
	return EndpointSliceLabels().AsSelectorPreValidated().Matches(labels.Set(obj.GetLabels()))
}

// EndpointToBeReflected filters out the endpoints targeting pods already running on the remote cluster.
func EndpointToBeReflected(endpoint *discoveryv1.Endpoint) bool {
	return !pointer.StringEqual(endpoint.NodeName, &LiqoNodeName)
}

// RemoteEndpointSlice forges the apply patch for the reflected endpointslice, given the local one.
func RemoteEndpointSlice(local *discoveryv1.EndpointSlice, targetNamespace string,
	translator EndpointTranslator) *discoveryv1apply.EndpointSliceApplyConfiguration {
	return discoveryv1apply.EndpointSlice(local.GetName(), targetNamespace).
		WithLabels(local.GetLabels()).WithLabels(ReflectionLabels()).
		WithLabels(EndpointSliceLabels()).WithAnnotations(local.GetAnnotations()).
		WithAddressType(local.AddressType).
		WithEndpoints(RemoteEndpointSliceEndpoints(local.Endpoints, translator)...).
		WithPorts(RemoteEndpointSlicePorts(local.Ports)...)
}

// RemoteEndpointSliceEndpoints forges the apply patch for the endpoints of the reflected endpointslice, given the local ones.
func RemoteEndpointSliceEndpoints(locals []discoveryv1.Endpoint,
	translator EndpointTranslator) []*discoveryv1apply.EndpointApplyConfiguration {
	var remotes []*discoveryv1apply.EndpointApplyConfiguration

	for i := range locals {
		if !EndpointToBeReflected(&locals[i]) {
			// Skip the endpoints referring to the target node (as natively present).
			continue
		}

		local := locals[i].DeepCopy()
		conditions := &discoveryv1apply.EndpointConditionsApplyConfiguration{Ready: local.Conditions.Ready}

		remote := discoveryv1apply.Endpoint().
			WithAddresses(translator(local.Addresses)...).WithConditions(conditions).
			WithNodeName(LocalCluster.ClusterName).WithHints(RemoteEndpointHints(local.Hints)).
			WithTargetRef(RemoteObjectReference(local.TargetRef))
		remote.Hostname = local.Hostname
		remote.Zone = local.Zone

		remotes = append(remotes, remote)
	}

	return remotes
}

// RemoteEndpointSlicePorts forges the apply patch for the ports of the reflected endpointslice, given the local ones.
func RemoteEndpointSlicePorts(locals []discoveryv1.EndpointPort) []*discoveryv1apply.EndpointPortApplyConfiguration {
	var remotes []*discoveryv1apply.EndpointPortApplyConfiguration

	for i := range locals {
		// DeepCopy the local object, to avoid mutating the cache.
		local := locals[i].DeepCopy()
		remotes = append(remotes, &discoveryv1apply.EndpointPortApplyConfiguration{
			Name: local.Name, Port: local.Port, Protocol: local.Protocol, AppProtocol: local.AppProtocol,
		})
	}

	return remotes
}

// RemoteEndpointHints forges the apply patch for the endpoint hints of the reflected endpointslice, given the local ones.
func RemoteEndpointHints(local *discoveryv1.EndpointHints) *discoveryv1apply.EndpointHintsApplyConfiguration {
	if local == nil {
		return nil
	}

	hints := discoveryv1apply.EndpointHints()
	for _, zone := range local.ForZones {
		hints.WithForZones(discoveryv1apply.ForZone().WithName(zone.Name))
	}
	return hints
}
