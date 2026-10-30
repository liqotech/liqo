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

package shadowpod

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// quotaReleaser is a controller that releases the quota accounted for a ShadowPod as soon as it reaches
// a terminal phase, without waiting for the next cache refresh.
type quotaReleaser struct {
	client    client.Client
	validator *Validator
}

// SetupQuotaReleaser registers a controller that releases the quota accounted for terminal ShadowPods
// as soon as they reach a terminal phase (Succeeded or Failed).
func (spv *Validator) SetupQuotaReleaser(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("shadowpod-quota-releaser").
		For(&offloadingv1beta1.ShadowPod{}, builder.WithPredicates(terminalPhasePredicate())).
		Complete(&quotaReleaser{client: mgr.GetClient(), validator: spv})
}

// Reconcile releases the quota accounted for the ShadowPod, if it reached a terminal phase.
func (r *quotaReleaser) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	shadowPod := &offloadingv1beta1.ShadowPod{}
	if err := r.client.Get(ctx, req.NamespacedName, shadowPod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.validator.releaseQuotaOfTerminatedShadowPod(shadowPod)
	return ctrl.Result{}, nil
}

// terminalPhasePredicate triggers a reconciliation only when a ShadowPod enters a terminal phase.
func terminalPhasePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			shadowPod, ok := e.Object.(*offloadingv1beta1.ShadowPod)
			return ok && isTerminalPodPhase(shadowPod.Status.Phase)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldShadowPod, ok := e.ObjectOld.(*offloadingv1beta1.ShadowPod)
			if !ok {
				return false
			}
			newShadowPod, ok := e.ObjectNew.(*offloadingv1beta1.ShadowPod)
			if !ok {
				return false
			}
			return !isTerminalPodPhase(oldShadowPod.Status.Phase) && isTerminalPodPhase(newShadowPod.Status.Phase)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// releaseQuotaOfTerminatedShadowPod releases the quota accounted for the given ShadowPod.
// It is a no-op if the ShadowPod is not in a terminal phase, it has no creator label, or no
// PeeringInfo is recorded in cache for its creator (the next cache refresh will align it anyway).
func (spv *Validator) releaseQuotaOfTerminatedShadowPod(shadowPod *offloadingv1beta1.ShadowPod) {
	if !isTerminalPodPhase(shadowPod.Status.Phase) {
		return
	}

	creatorName, found := shadowPod.Labels[consts.CreatorLabelKey]
	if !found {
		return
	}

	peeringInfo, found := spv.PeeringCache.getPeeringInfo(creatorName)
	if !found {
		return
	}

	nsname := types.NamespacedName{Name: shadowPod.GetName(), Namespace: shadowPod.GetNamespace()}
	peeringInfo.mu.Lock()
	defer peeringInfo.mu.Unlock()
	peeringInfo.releaseTerminatedShadowPod(nsname)
}
