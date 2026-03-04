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

package remoterenwercontroller

import (
	"bytes"
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/consts"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/events"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// RemoteRenewerReconciler reconciles a Renew object by syncing the Tenant's
// AuthParams to the Renew status. The Tenant controller handles certificate
// signing and renewal; this controller only propagates the results back to
// the consumer cluster via the CRD replicator.
type RemoteRenewerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	NamespaceManager tenantnamespace.Manager
	recorder         record.EventRecorder
}

// NewRemoteRenewerReconciler returns a new RemoteRenewerReconciler.
func NewRemoteRenewerReconciler(cl client.Client, s *runtime.Scheme,
	namespaceManager tenantnamespace.Manager,
	recorder record.EventRecorder) *RemoteRenewerReconciler {
	return &RemoteRenewerReconciler{
		Client: cl,
		Scheme: s,

		NamespaceManager: namespaceManager,
		recorder:         recorder,
	}
}

//+kubebuilder:rbac:groups=authentication.liqo.io,resources=renews,verbs=get;list;watch;update;patch;delete;create
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=renews/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=tenants,verbs=get;list;watch
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=tenants/status,verbs=get

// Reconcile syncs the Tenant's AuthParams to the Renew's status.
// When the Tenant controller renews a certificate, the updated AuthParams
// appear in Tenant.Status.AuthParams. This controller copies them to
// Renew.Status.AuthParams for replication back to the consumer cluster.
func (r *RemoteRenewerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var renew authv1beta1.Renew
	if err := r.Get(ctx, req.NamespacedName, &renew); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip if the Renew is being deleted.
	if !renew.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	tenantNamespace, err := r.NamespaceManager.GetNamespace(ctx, renew.Spec.ConsumerClusterID)
	if err != nil {
		klog.Errorf("Unable to get tenant namespace for Renew %q: %s", req.NamespacedName, err)
		events.EventWithOptions(r.recorder, &renew, fmt.Sprintf("Failed to get tenant namespace: %s", err),
			&events.Option{EventType: events.Error, Reason: "TenantNamespaceNotFound"})
		return ctrl.Result{}, err
	}

	if tenantNamespace.Name != renew.Namespace {
		klog.V(4).Infof("Skipping Renew %q as it's not in the tenant namespace %q", req.NamespacedName, tenantNamespace.Name)
		events.EventWithOptions(r.recorder, &renew, fmt.Sprintf("Skipping renewal as it's not in tenant namespace %s", tenantNamespace.Name),
			&events.Option{EventType: events.Warning, Reason: "WrongNamespace"})
		return ctrl.Result{}, nil
	}

	tenant, err := getters.GetTenantByClusterID(ctx, r.Client, renew.Spec.ConsumerClusterID, tenantNamespace.Name)
	if err != nil {
		klog.Errorf("Unable to get Tenant for Renew %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// If the Tenant's AuthParams are not yet populated, nothing to sync.
	if tenant.Status.AuthParams == nil || len(tenant.Status.AuthParams.SignedCRT) == 0 {
		klog.V(4).Infof("Tenant %q AuthParams not yet available for Renew %q", tenant.Name, req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// If the Renew's AuthParams are already in sync with the Tenant's, skip.
	if renew.Status.AuthParams != nil &&
		bytes.Equal(renew.Status.AuthParams.SignedCRT, tenant.Status.AuthParams.SignedCRT) {
		return ctrl.Result{}, nil
	}

	// Copy the Tenant's AuthParams to the Renew's status.
	renew.Status.AuthParams = tenant.Status.AuthParams
	if err := r.Status().Update(ctx, &renew); err != nil {
		klog.Errorf("Failed to update Renew status for %q: %s", req.NamespacedName, err)
		events.EventWithOptions(r.recorder, &renew, fmt.Sprintf("Failed to update Renew status: %s", err),
			&events.Option{EventType: events.Error, Reason: "RenewStatusUpdateFailed"})
		return ctrl.Result{}, err
	}

	klog.V(4).Infof("Synced AuthParams from Tenant %q to Renew %q", tenant.Name, req.NamespacedName)
	events.Event(r.recorder, &renew, "Synced AuthParams from Tenant to Renew")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RemoteRenewerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	replicatedFilter, err := predicate.LabelSelectorPredicate(reflection.ReplicatedResourcesLabelSelector())
	if err != nil {
		return err
	}

	controlPlaneFilter := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		renew, ok := obj.(*authv1beta1.Renew)
		if !ok {
			return false
		}
		return renew.Spec.IdentityType == authv1beta1.ControlPlaneIdentityType
	})

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlRenewRemote).
		For(&authv1beta1.Renew{}, builder.WithPredicates(predicate.And(replicatedFilter, controlPlaneFilter))).
		Watches(&authv1beta1.Tenant{}, handler.EnqueueRequestsFromMapFunc(r.renewsEnqueuer())).
		Complete(r)
}

// renewsEnqueuer returns a function that maps Tenant changes to reconciliation
// requests for the associated ControlPlane Renew objects.
func (r *RemoteRenewerReconciler) renewsEnqueuer() func(ctx context.Context, obj client.Object) []reconcile.Request {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		tenant, ok := obj.(*authv1beta1.Tenant)
		if !ok {
			return nil
		}

		// List replicated Renew objects in the tenant namespace.
		var renewList authv1beta1.RenewList
		if err := r.List(ctx, &renewList, client.InNamespace(tenant.Namespace)); err != nil {
			klog.Errorf("Failed to list Renew objects for Tenant %q: %v", tenant.Name, err)
			return nil
		}

		var reqs []reconcile.Request
		for i := range renewList.Items {
			rn := &renewList.Items[i]
			if rn.Spec.IdentityType != authv1beta1.ControlPlaneIdentityType {
				continue
			}
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      rn.Name,
					Namespace: rn.Namespace,
				},
			})
		}

		return reqs
	}
}
