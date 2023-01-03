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

package foreignclusteroperator

import (
	"context"

	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

type desiredPeeringPhase string

const (
	desiredPeeringPhasePeering   desiredPeeringPhase = "Peering"
	desiredPeeringPhaseUnpeering desiredPeeringPhase = "Unpeering"
)

// getDesiredOutgoingPeeringState returns the desired state for the outgoing peering basing on the ForeignCluster resource.
func (r *ForeignClusterReconciler) getDesiredOutgoingPeeringState(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) desiredPeeringPhase {
	outgoingPeeringEnabled, err := r.isOutgoingPeeringEnabled(ctx, foreignCluster)
	if err != nil {
		klog.Error(err)
		return desiredPeeringPhaseUnpeering
	}

	remoteNamespace := foreignCluster.Status.TenantNamespace.Remote
	if remoteNamespace != "" && outgoingPeeringEnabled {
		return desiredPeeringPhasePeering
	}
	return desiredPeeringPhaseUnpeering
}

func (r *ForeignClusterReconciler) isOutgoingPeeringEnabled(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) (bool, error) {
	if !foreignCluster.GetDeletionTimestamp().IsZero() {
		return false, nil
	}

	switch foreignCluster.Spec.OutgoingPeeringEnabled {
	case discoveryv1alpha1.PeeringEnabledNo:
		return false, nil
	case discoveryv1alpha1.PeeringEnabledYes:
		return true, nil
	case discoveryv1alpha1.PeeringEnabledAuto:
		if !r.AutoJoin {
			return false, nil
		}

		discoveryType := foreignclusterutils.GetDiscoveryType(foreignCluster)
		switch discoveryType {
		case discovery.LanDiscovery:
			return true, nil
		case discovery.ManualDiscovery, discovery.IncomingPeeringDiscovery:
			return false, nil
		}
	}

	return false, nil
}
