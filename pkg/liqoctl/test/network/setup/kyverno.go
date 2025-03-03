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

package setup

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
)

// KyvernoPolicyGroupVersionResource specifies the group version resource used to register the objects.
var KyvernoPolicyGroupVersionResource = schema.GroupVersionResource{Group: "kyverno.io", Version: "v1", Resource: "policies"}

// KyvernoPolicyKind is the kind of the Kyverno policy.
const KyvernoPolicyKind = "Policy"

// CreatePolicy creates the Kyverno policies.
func CreatePolicy(ctx context.Context, cl *client.Client) error {
	policy := ForgeKyvernoPodAntiaffinityPolicy(cl.ConsumerName, false)
	if _, err := cl.ConsumerDynamic.Resource(KyvernoPolicyGroupVersionResource).
		Namespace(NamespaceName).Create(ctx, policy, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("consumer failed to create policy: %w", err)
	}

	policy = ForgeKyvernoPodAntiaffinityPolicy(cl.ConsumerName, true)
	if _, err := cl.ConsumerDynamic.Resource(KyvernoPolicyGroupVersionResource).
		Namespace(NamespaceName).Create(ctx, policy, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("consumer failed to create policy: %w", err)
	}

	for k := range cl.Providers {
		policy := ForgeKyvernoPodAntiaffinityPolicy(k, false)
		if _, err := cl.ProvidersDynamic[k].Resource(KyvernoPolicyGroupVersionResource).
			Namespace(NamespaceName).Create(ctx, policy, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("provider %q failed to create policy: %w", k, err)
		}
		policy = ForgeKyvernoPodAntiaffinityPolicy(k, true)
		if _, err := cl.ProvidersDynamic[k].Resource(KyvernoPolicyGroupVersionResource).
			Namespace(NamespaceName).Create(ctx, policy, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("provider %q failed to create policy: %w", k, err)
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
	name := "pod-antiaffinity"
	if hostnetwork {
		deploymentName = DeploymentName + "-host"
		name += "-host"
	}

	policy.SetName(name)
	policy.SetNamespace(NamespaceName)

	policy.Object["spec"] = map[string]interface{}{
		"rules": []map[string]interface{}{
			{
				"name": name,
				"match": map[string]interface{}{
					"any": []map[string]interface{}{
						{"resources": map[string]interface{}{
							"kinds": []string{"Pod"},
							"selector": map[string]interface{}{
								"matchLabels": map[string]string{
									PodLabelAppCluster: deploymentName + "-" + suffix,
								},
							},
						}},
					},
				},
				"mutate": map[string]interface{}{
					"patchStrategicMerge": forgeRawPatchStrategicMerge(deploymentName+"-"+suffix, hostnetwork),
				},
			},
		},
	}
	return policy
}

func forgeRawPatchStrategicMerge(labelValue string, hostnetwork bool) map[string]interface{} {
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"+(affinity)": map[string]interface{}{
				"+(podAntiAffinity)": map[string]interface{}{
					"+(preferredDuringSchedulingIgnoredDuringExecution)": []map[string]interface{}{
						{
							"weight": 100,
							"podAffinityTerm": map[string]interface{}{
								"labelSelector": map[string]interface{}{
									"matchLabels": map[string]string{
										PodLabelAppCluster: labelValue,
									},
								},
								"topologyKey": "kubernetes.io/hostname",
							},
						},
					},
				},
			},
			"+(hostNetwork)": hostnetwork,
		},
	}
}
