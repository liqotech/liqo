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

package resourcerequestoperator

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

type resourceRequestPhase string

const (
	allowResourceRequestPhase    resourceRequestPhase = "Allow"
	denyResourceRequestPhase     resourceRequestPhase = "Deny"
	deletingResourceRequestPhase resourceRequestPhase = "Deleting"
)

// getResourceRequestPhase returns the phase associated with a resource request. It is:
// * "Deleting" if the deletion timestamp is set or the related offer has been withdrawn.
// * "Allow" if the incoming peering is enabled in the ForeignCluster or through the command line parameter.
// * "Deny" in the other cases (no ForeignCluster, incoming peering disabled, ...)
func (r *ResourceRequestReconciler) getResourceRequestPhase(
	foreignCluster *discoveryv1alpha1.ForeignCluster,
	resourceRequest *discoveryv1alpha1.ResourceRequest) (resourceRequestPhase, error) {
	if !resourceRequest.GetDeletionTimestamp().IsZero() || !resourceRequest.Spec.WithdrawalTimestamp.IsZero() {
		return deletingResourceRequestPhase, nil
	}

	if foreignclusterutils.AllowIncomingPeering(foreignCluster, r.EnableIncomingPeering) {
		return allowResourceRequestPhase, nil
	}
	return denyResourceRequestPhase, nil
}
