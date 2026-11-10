// Copyright 2019-2026 The Liqo Authors
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
	"slices"

	corev1 "k8s.io/api/core/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/pointer"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// nodePortUnset -> the value representing an unset NodePort.
const nodePortUnset = 0

// RemoteService forges the apply patch for the reflected service, given the local one.
func RemoteService(local *corev1.Service, targetNamespace string, enableLoadBalancer bool, remoteRealLoadBalancerClassName string,
	forgingOpts *ForgingOpts) *corev1apply.ServiceApplyConfiguration {
	return corev1apply.Service(local.GetName(), targetNamespace).
		WithLabels(FilterNotReflected(local.GetLabels(), forgingOpts.LabelsNotReflected)).WithLabels(ReflectionLabels()).
		WithAnnotations(FilterNotReflected(local.GetAnnotations(), forgingOpts.AnnotationsNotReflected)).
		WithSpec(RemoteServiceSpec(local.Spec.DeepCopy(), getForceRemoteNodePort(local), enableLoadBalancer, remoteRealLoadBalancerClassName))
}

// RemoteServiceSpec forges the apply patch for the specs of the reflected service, given the local ones.
// It expects the local object to be a deepcopy, as it is mutated.
func RemoteServiceSpec(local *corev1.ServiceSpec, forceRemoteNodePort,
	enableLoadBalancer bool, remoteRealLoadBalancerClassName string) *corev1apply.ServiceSpecApplyConfiguration {
	remote := corev1apply.ServiceSpec().
		WithType(local.Type).WithSelector(local.Selector).
		WithPorts(RemoteServicePorts(local.Ports, forceRemoteNodePort)...).
		WithExternalName(local.ExternalName)

	// The additional fields are set manually instead of using the "With" functions,
	// to avoid issues if not set in the local object and thus nil. This requires the
	// local object to be a deepcopy to avoid mutating the original from the cache.
	remote.AllocateLoadBalancerNodePorts = local.AllocateLoadBalancerNodePorts
	remote.ExternalTrafficPolicy = &local.ExternalTrafficPolicy
	remote.InternalTrafficPolicy = local.InternalTrafficPolicy
	remote.IPFamilyPolicy, remote.IPFamilies = remoteIPFamilies(local)
	remote.LoadBalancerSourceRanges = local.LoadBalancerSourceRanges
	remote.PublishNotReadyAddresses = &local.PublishNotReadyAddresses
	remote.SessionAffinity = &local.SessionAffinity

	if local.ClusterIP == corev1.ClusterIPNone {
		remote.ClusterIP = pointer.String(corev1.ClusterIPNone)
	}

	if local.Type == corev1.ServiceTypeLoadBalancer {
		if enableLoadBalancer {
			remote.WithLoadBalancerClass(remoteRealLoadBalancerClassName)
		}
	}

	return remote
}

// remoteIPFamilies returns the IP family policy and the IP families of the reflected service. The Liqo network fabric is IPv4 only,
// and the reflected endpointslices are IPv4: if the local service has an IPv4 family, IPv4 is the primary family of the remote
// service, regardless of its position in the local families. This prevents the remote service from getting an IPv6 primary
// ClusterIP, which would have no endpoints, on IPv6-primary dual-stack clusters.
//   - RequireDualStack is honored: the remote service is dual-stack, IPv4 first (its IPv6 ClusterIP has no endpoints).
//   - Otherwise, the remote service is IPv4 single-stack: a secondary IPv6 ClusterIP would have no endpoints. The policy must be
//     SingleStack as well, otherwise the apiserver rejects releasing the secondary IP family of a service that was previously
//     reflected as dual-stack.
//
// Otherwise (no IPv4 family, or ExternalName), the policy is propagated as is and the families are left unset, leaving the
// choice to the remote cluster.
func remoteIPFamilies(local *corev1.ServiceSpec) (*corev1.IPFamilyPolicy, []corev1.IPFamily) {
	if !slices.Contains(local.IPFamilies, corev1.IPv4Protocol) {
		return local.IPFamilyPolicy, nil
	}
	if local.IPFamilyPolicy != nil && *local.IPFamilyPolicy == corev1.IPFamilyPolicyRequireDualStack {
		policy := corev1.IPFamilyPolicyRequireDualStack
		return &policy, []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}
	}
	policy := corev1.IPFamilyPolicySingleStack
	return &policy, []corev1.IPFamily{corev1.IPv4Protocol}
}

// RemoteServicePorts forges the apply patch for the ports of the reflected service, given the local ones.
func RemoteServicePorts(locals []corev1.ServicePort, forceRemoteNodePort bool) []*corev1apply.ServicePortApplyConfiguration {
	var remotes []*corev1apply.ServicePortApplyConfiguration

	for _, local := range locals {
		remote := corev1apply.ServicePort().WithName(local.Name).WithPort(local.Port).
			WithTargetPort(local.TargetPort).WithProtocol(local.Protocol)

		if local.NodePort == nodePortUnset {
			// Ensure the nodeport is unset in case it is removed, to allow
			// switching from a NodePort to a ClusterIP service.
			remote.WithNodePort(nodePortUnset)
		}

		if forceRemoteNodePort {
			remote.WithNodePort(local.NodePort)
		}

		if local.AppProtocol != nil {
			// Need to check to avoid dereferencing a nil pointer.
			remote.WithAppProtocol(*local.AppProtocol)
		}
		remotes = append(remotes, remote)
	}

	return remotes
}

func getForceRemoteNodePort(local *corev1.Service) bool {
	val, ok := local.Annotations[liqoconst.ForceRemoteNodePortAnnotationKey]
	return ok && val == "true"
}
