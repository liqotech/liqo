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

// KyvernoPolicyGroupVersionResource specifies the group version resource used to register the objects.
var KyvernoPolicyGroupVersionResource = schema.GroupVersionResource{Group: "kyverno.io", Version: "v1", Resource: "policies"}

// KyvernoPolicyKind is the kind of the Kyverno policy.
const KyvernoPolicyKind = "Policy"

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

// CreatePolicy creates the Kyverno policies.
func CreatePolicy(ctx context.Context, cl *client.Client, opts *flags.Options) error {
	var kyvernoNotInstalled bool
	printer := opts.Topts.LocalFactory.Printer

	if IsKyvernoAvailable(ctx, cl.ConsumerDynamic) {
		if err := createPolicyForCluster(ctx, cl.ConsumerDynamic, cl.ConsumerName, "consumer"); err != nil {
			return err
		}
	} else {
		kyvernoNotInstalled = true
		printer.Logger.Warn("Kyverno not available on consumer, skipping policy creation.")
	}

	for k := range cl.Providers {
		if IsKyvernoAvailable(ctx, cl.ProvidersDynamic[k]) {
			if err := createPolicyForCluster(ctx, cl.ProvidersDynamic[k], k, fmt.Sprintf("provider %q", k)); err != nil {
				return err
			}
		} else {
			kyvernoNotInstalled = true
			printer.Logger.Warn(fmt.Sprintf("Kyverno not available on provider %q, skipping policy creation.", k))
		}
	}

	if kyvernoNotInstalled {
		printer.Logger.Warn("Pods may not be scheduled on every node. Install Kyverno on all clusters for comprehensive tests.")
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
