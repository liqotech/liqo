// Copyright 2019-2025 The Liqo Authors
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

package shadowendpointslicectrl

import (
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	discoveryv1apply "k8s.io/client-go/applyconfigurations/discovery/v1"
)

// EndpointSliceApply forges the apply patch for the endpointslice.
func EndpointSliceApply(eps *discoveryv1.EndpointSlice) *discoveryv1apply.EndpointSliceApplyConfiguration {
	return discoveryv1apply.EndpointSlice(eps.GetName(), eps.GetNamespace()).
		WithLabels(eps.GetLabels()).
		WithAnnotations(eps.GetAnnotations()).
		WithAddressType(eps.AddressType).
		WithEndpoints(EndpointsApply(eps.Endpoints)...).
		WithPorts(EndpointPortsApply(eps.Ports)...)
}

// EndpointsApply forges the apply patch for the endpoints of the reflected endpointslice, given the local ones.
func EndpointsApply(endpoints []discoveryv1.Endpoint) []*discoveryv1apply.EndpointApplyConfiguration {
	var output []*discoveryv1apply.EndpointApplyConfiguration

	for i := range endpoints {
		ep := &endpoints[i]
		conditions := &discoveryv1apply.EndpointConditionsApplyConfiguration{Ready: ep.Conditions.Ready}

		epApply := discoveryv1apply.Endpoint().
			WithAddresses(ep.Addresses...).
			WithConditions(conditions).
			WithTargetRef(ObjectReferenceApply(ep.TargetRef)).
			WithHints(EndpointHintsApply(ep.Hints))
		epApply.NodeName = ep.NodeName
		epApply.Hostname = ep.Hostname
		epApply.Zone = ep.Zone

		output = append(output, epApply)
	}

	return output
}

// ObjectReferenceApply forges the apply patch for a reflected ObjectReference.
func ObjectReferenceApply(ref *corev1.ObjectReference) *corev1apply.ObjectReferenceApplyConfiguration {
	if ref == nil {
		return nil
	}

	return corev1apply.ObjectReference().
		WithAPIVersion(ref.APIVersion).WithFieldPath(ref.FieldPath).
		WithKind(ref.Kind).WithName(ref.Name).WithNamespace(ref.Namespace).
		WithResourceVersion(ref.ResourceVersion).WithUID(ref.UID)
}

// EndpointHintsApply forges the apply patch for the endpoint hints of the reflected endpointslice, given the local ones.
func EndpointHintsApply(hints *discoveryv1.EndpointHints) *discoveryv1apply.EndpointHintsApplyConfiguration {
	if hints == nil {
		return nil
	}

	output := discoveryv1apply.EndpointHints()
	for _, zone := range hints.ForZones {
		output.WithForZones(discoveryv1apply.ForZone().WithName(zone.Name))
	}
	return output
}

// EndpointPortsApply forges the apply patch for the ports of the reflected endpointslice, given the local ones.
func EndpointPortsApply(ports []discoveryv1.EndpointPort) []*discoveryv1apply.EndpointPortApplyConfiguration {
	var output []*discoveryv1apply.EndpointPortApplyConfiguration

	for i := range ports {
		port := &ports[i]
		output = append(output, &discoveryv1apply.EndpointPortApplyConfiguration{
			Name: port.Name, Port: port.Port, Protocol: port.Protocol, AppProtocol: port.AppProtocol,
		})
	}

	return output
}
