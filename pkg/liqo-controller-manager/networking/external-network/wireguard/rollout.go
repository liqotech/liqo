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

package wireguard

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway"
)

const (
	// DefaultRolloutRequeueInterval is the delay before a controller retries when
	// its rollout is gated by a peer.
	DefaultRolloutRequeueInterval = 5 * time.Second
)

// IsDeploymentRollingOut reports whether the Deployment is currently rolling out.
func IsDeploymentRollingOut(dep *appsv1.Deployment) bool {
	if dep == nil {
		return false
	}
	desired := dep.Status.Replicas
	if dep.Spec.Replicas != nil {
		desired = *dep.Spec.Replicas
	}
	if dep.Generation > dep.Status.ObservedGeneration {
		return true
	}
	return dep.Status.UpdatedReplicas < desired ||
		dep.Status.AvailableReplicas < desired ||
		dep.Status.UnavailableReplicas > 0
}

// ShouldDelayRollout checks whether the requested rollout of the gateway Deployment should be delayed because a peer Deployment
// for the same remote cluster is still stale or rolling out.
func ShouldDelayRollout(ctx context.Context, cl client.Client, remoteClusterID string,
	self types.NamespacedName, selfTemplateName, selfTemplateNamespace, desiredGeneration string) (delay bool, reason string, err error) {
	// If the source template is not known (e.g., manually created resources), we
	// cannot compare peers, so we proceed without gating.
	if remoteClusterID == "" || selfTemplateName == "" || selfTemplateNamespace == "" || desiredGeneration == "" {
		return false, "", nil
	}

	// We need to select all the peer deployments pointing to the same cluster-id and using the same template.
	selector := labels.SelectorFromSet(labels.Set{
		consts.RemoteClusterID:        remoteClusterID,
		consts.NetworkingComponentKey: gateway.GatewayComponentGateway,
	})

	var deps appsv1.DeploymentList
	if err := cl.List(ctx, &deps, &client.ListOptions{LabelSelector: selector}); err != nil {
		return false, "", fmt.Errorf("listing peer gateway deployments: %w", err)
	}

	selfKey := self.String()
	for i := range deps.Items {
		dep := &deps.Items[i]
		if dep.Namespace == self.Namespace && dep.Name == self.Name {
			continue
		}

		// Ignore peers generated from a different source template.
		annotations := dep.GetAnnotations()
		if annotations[consts.TemplateNameAnnotationKey] != selfTemplateName ||
			annotations[consts.TemplateNamespaceAnnotationKey] != selfTemplateNamespace {
			continue
		}

		peerKey := client.ObjectKeyFromObject(dep).String()
		peerGeneration := annotations[consts.TemplateGenerationAnnotationKey]

		// If a lower-named peer with a different generation is still waiting to roll. We must wait for it before we can proceed.
		if peerKey < selfKey && peerGeneration != desiredGeneration {
			return true, fmt.Sprintf("lower-named peer %s has stale generation %q (want %q)", peerKey, peerGeneration, desiredGeneration), nil
		}

		// Wait while the peer that comes before us is still rolling out.
		if IsDeploymentRollingOut(dep) {
			return true, fmt.Sprintf("peer %s is still rolling out", peerKey), nil
		}
	}

	return false, "", nil
}

func isDeploymentUpdateNeeded(existing, desired *appsv1.Deployment) bool {
	if existing == nil || desired == nil {
		return existing != desired
	}
	return !equality.Semantic.DeepEqual(existing.Spec, desired.Spec) ||
		!equality.Semantic.DeepEqual(existing.Labels, desired.Labels) ||
		!equality.Semantic.DeepEqual(existing.Annotations, desired.Annotations)
}

func getRemoteClusterID(obj client.Object) string {
	if obj == nil || obj.GetLabels() == nil {
		return ""
	}
	return obj.GetLabels()[consts.RemoteClusterID]
}
