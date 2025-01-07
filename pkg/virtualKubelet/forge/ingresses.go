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

package forge

import (
	netv1 "k8s.io/api/networking/v1"
	netv1apply "k8s.io/client-go/applyconfigurations/networking/v1"

	"github.com/liqotech/liqo/pkg/utils/maps"
)

// RemoteIngress forges the apply patch for the reflected ingress, given the local one.
func RemoteIngress(local *netv1.Ingress, targetNamespace string, enableIngress bool, remoteRealIngressClassName string,
	forgingOpts *ForgingOpts) *netv1apply.IngressApplyConfiguration {
	return netv1apply.Ingress(local.GetName(), targetNamespace).
		WithLabels(FilterNotReflected(local.GetLabels(), forgingOpts.LabelsNotReflected)).WithLabels(ReflectionLabels()).
		WithAnnotations(FilterNotReflected(FilterIngressAnnotations(local.GetAnnotations()), forgingOpts.AnnotationsNotReflected)).
		WithSpec(RemoteIngressSpec(local.Spec.DeepCopy(), enableIngress, remoteRealIngressClassName))
}

// FilterIngressAnnotations filters the ingress annotations to be reflected, removing the ingress class annotation.
func FilterIngressAnnotations(local map[string]string) map[string]string {
	return maps.Filter(local, maps.FilterBlacklist("kubernetes.io/ingress.class"))
}

// RemoteIngressSpec forges the apply patch for the specs of the reflected ingress, given the local one.
// It expects the local object to be a deepcopy, as it is mutated.
func RemoteIngressSpec(local *netv1.IngressSpec, enableIngress bool, remoteRealIngressClassName string) *netv1apply.IngressSpecApplyConfiguration {
	ret := netv1apply.IngressSpec().
		WithDefaultBackend(RemoteIngressBackend(local.DefaultBackend)).
		WithRules(RemoteIngressRules(local.Rules)...).
		WithTLS(RemoteIngressTLS(local.TLS)...)
	if enableIngress {
		ret.WithIngressClassName(remoteRealIngressClassName)
	}
	return ret
}

// RemoteIngressBackend forges the apply patch for the backend of the reflected ingress, given the local one.
func RemoteIngressBackend(local *netv1.IngressBackend) *netv1apply.IngressBackendApplyConfiguration {
	if local == nil {
		return nil
	}

	return netv1apply.IngressBackend().
		WithResource(RemoteTypedLocalObjectReference(local.Resource)).
		WithService(RemoteIngressService(local.Service))
}

// RemoteIngressService forges the apply patch for the service of the reflected ingress, given the local one.
func RemoteIngressService(local *netv1.IngressServiceBackend) *netv1apply.IngressServiceBackendApplyConfiguration {
	if local == nil {
		return nil
	}
	return netv1apply.IngressServiceBackend().
		WithName(local.Name).
		WithPort(netv1apply.ServiceBackendPort().
			WithName(local.Port.Name).
			WithNumber(local.Port.Number))
}

// RemoteIngressRules forges the apply patch for the rules of the reflected ingress, given the local ones.
func RemoteIngressRules(local []netv1.IngressRule) []*netv1apply.IngressRuleApplyConfiguration {
	remote := make([]*netv1apply.IngressRuleApplyConfiguration, len(local))
	for i := range local {
		remote[i] = netv1apply.IngressRule().
			WithHost(local[i].Host).
			WithHTTP(RemoteIngressHTTP(local[i].HTTP))
	}
	return remote
}

// RemoteIngressHTTP forges the apply patch for the HTTPIngressRuleValue of the reflected ingress, given the local one.
func RemoteIngressHTTP(local *netv1.HTTPIngressRuleValue) *netv1apply.HTTPIngressRuleValueApplyConfiguration {
	if local == nil {
		return nil
	}
	return netv1apply.HTTPIngressRuleValue().
		WithPaths(RemoteIngressPaths(local.Paths)...)
}

// RemoteIngressPaths forges the apply patch for the paths of the reflected ingress, given the local ones.
func RemoteIngressPaths(local []netv1.HTTPIngressPath) []*netv1apply.HTTPIngressPathApplyConfiguration {
	remote := make([]*netv1apply.HTTPIngressPathApplyConfiguration, len(local))
	for i := range local {
		remote[i] = netv1apply.HTTPIngressPath().
			WithPath(local[i].Path).
			WithBackend(RemoteIngressBackend(&local[i].Backend))

		remote[i].PathType = local[i].PathType
	}
	return remote
}

// RemoteIngressTLS forges the apply patch for the TLS configs of the reflected ingress, given the local ones.
func RemoteIngressTLS(local []netv1.IngressTLS) []*netv1apply.IngressTLSApplyConfiguration {
	remote := make([]*netv1apply.IngressTLSApplyConfiguration, len(local))
	for i := range local {
		remote[i] = netv1apply.IngressTLS().
			WithHosts(local[i].Hosts...).
			WithSecretName(local[i].SecretName)
	}
	return remote
}
