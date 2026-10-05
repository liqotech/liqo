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

package setup

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
)

const (
	// kyvernoPolicyVersion is the API version used for Kyverno MutatingAdmissionPolicy-based policies.
	kyvernoPolicyVersion = "v1"
	// kyvernoExpressionKey is the key used for CEL expression fields in Kyverno policies.
	kyvernoExpressionKey = "expression"
)

// KyvernoPolicyGroupVersionResource specifies the group version resource used to register the objects.
// This API is available starting from Kyverno v1.13 (MutatingAdmissionPolicy-based NamespacedMutatingPolicy).
// Older Kyverno releases do not expose this resource and are not supported by the network tests.
var KyvernoPolicyGroupVersionResource = schema.GroupVersionResource{
	Group:    "policies.kyverno.io",
	Version:  kyvernoPolicyVersion,
	Resource: "namespacedmutatingpolicies",
}

// KyvernoPolicyKind is the kind of the Kyverno policy.
const KyvernoPolicyKind = "NamespacedMutatingPolicy"

// IsKyvernoAvailable checks if Kyverno is available.
func IsKyvernoAvailable(ctx context.Context, cl *dynamic.DynamicClient) bool {
	_, err := cl.Resource(KyvernoPolicyGroupVersionResource).
		Namespace(NamespaceName).List(ctx, metav1.ListOptions{})
	return err == nil
}

// createPolicyForCluster creates Kyverno policies for a specific cluster.
func createPolicyForCluster(ctx context.Context, dynClient dynamic.Interface, clusterName, clusterType string) error {
	policy := ForgeKyvernoPodAntiaffinityPolicy(clusterName, false)
	if _, err := dynClient.Resource(KyvernoPolicyGroupVersionResource).
		Namespace(NamespaceName).Create(ctx, policy, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("%s failed to create policy: %w", clusterType, err)
	}

	policy = ForgeKyvernoPodAntiaffinityPolicy(clusterName, true)
	if _, err := dynClient.Resource(KyvernoPolicyGroupVersionResource).
		Namespace(NamespaceName).Create(ctx, policy, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("%s failed to create policy: %w", clusterType, err)
	}

	return nil
}

const (
	kyvernoPodAntiaffinityName = "pod-antiaffinity"
)

// CreatePolicy removes stale Kyverno mutation policies left by previous liqoctl versions.
// Pod anti-affinity and hostNetwork are now baked into the Deployment specs, so no new
// Kyverno policies need to be created.
func CreatePolicy(ctx context.Context, cl *client.Client, opts *flags.Options) error {
	names := []string{kyvernoPodAntiaffinityName, kyvernoPodAntiaffinityName + "-host"}

	deleteStale := func(dyn *dynamic.DynamicClient, clusterType string) error {
		if !IsKyvernoAvailable(ctx, dyn) {
			return nil
		}
		for _, name := range names {
			if err := dyn.Resource(KyvernoPolicyGroupVersionResource).Namespace(NamespaceName).
				Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("error deleting stale Kyverno policy %q on %s: %w", name, clusterType, err)
			}
		}
		return nil
	}

	if err := deleteStale(cl.ConsumerDynamic, "consumer"); err != nil {
		return err
	}
	for k := range cl.Providers {
		if err := deleteStale(cl.ProvidersDynamic[k], fmt.Sprintf("provider %q", k)); err != nil {
			return err
		}
	}
	return nil
}

// ForgeKyvernoPodAntiaffinityPolicy creates a Kyverno policy that enforces pod anti-affinity.
func ForgeKyvernoPodAntiaffinityPolicy(suffix string, hostnetwork bool) *unstructured.Unstructured {
	policy := &unstructured.Unstructured{}

	policy.SetKind(KyvernoPolicyKind)
	policy.SetAPIVersion(fmt.Sprintf("%s/%s", KyvernoPolicyGroupVersionResource.Group, KyvernoPolicyGroupVersionResource.Version))

	deploymentName := DeploymentName
	name := kyvernoPodAntiaffinityName
	if hostnetwork {
		deploymentName = DeploymentName + "-host"
		name += "-host"
	}

	policy.SetName(name)
	policy.SetNamespace(NamespaceName)

	labelValue := deploymentName + "-" + suffix

	policy.Object["spec"] = map[string]interface{}{
		"matchConstraints": map[string]interface{}{
			"resourceRules": []map[string]interface{}{
				{
					"apiGroups":   []string{""},
					"apiVersions": []string{kyvernoPolicyVersion},
					"operations":  []string{"CREATE"},
					"resources":   []string{"pods"},
				},
			},
		},
		"matchConditions": []map[string]interface{}{
			{
				"name":               "match-app-cluster-label",
				kyvernoExpressionKey: fmt.Sprintf("object.metadata.?labels['%s'].orValue('') == '%s'", PodLabelAppCluster, labelValue),
			},
		},
		"mutations": []map[string]interface{}{
			{
				"patchType": "ApplyConfiguration",
				"applyConfiguration": map[string]interface{}{
					kyvernoExpressionKey: forgeApplyConfigurationExpression(labelValue, hostnetwork),
				},
			},
		},
	}
	return policy
}

// forgeApplyConfigurationExpression builds the CEL ApplyConfiguration expression that
// enforces pod anti-affinity (and optionally hostNetwork) on the matched pods.
func forgeApplyConfigurationExpression(labelValue string, hostnetwork bool) string {
	hostNetwork := "false"
	if hostnetwork {
		hostNetwork = "true"
	}
	return fmt.Sprintf(`Object{
	spec: Object.spec{
		hostNetwork: %s,
		affinity: Object.spec.affinity{
			podAntiAffinity: Object.spec.affinity.podAntiAffinity{
				preferredDuringSchedulingIgnoredDuringExecution: [
					Object.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution{
						weight: 100,
						podAffinityTerm: Object.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution.podAffinityTerm{
							labelSelector: Object.spec.affinity.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution.podAffinityTerm.labelSelector{
								matchLabels: {"%s": "%s"}
							},
							topologyKey: "kubernetes.io/hostname"
						}
					}
				]
			}
		}
	}
}`, hostNetwork, PodLabelAppCluster, labelValue)
}
