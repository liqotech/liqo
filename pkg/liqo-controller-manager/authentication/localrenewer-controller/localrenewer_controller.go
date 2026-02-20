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

package localrenewercontroller

import (
	"bytes"
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/events"
)

// LocalRenewerReconciler reconciles an Identity object.
type LocalRenewerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	LocalClusterID liqov1beta1.ClusterID
	recorder       record.EventRecorder
}

// NewLocalRenewerReconciler returns a new LocalRenewerReconciler.
func NewLocalRenewerReconciler(cl client.Client, s *runtime.Scheme,
	localClusterID liqov1beta1.ClusterID,
	recorder record.EventRecorder) *LocalRenewerReconciler {
	return &LocalRenewerReconciler{
		Client:         cl,
		Scheme:         s,
		LocalClusterID: localClusterID,
		recorder:       recorder,
	}
}

//+kubebuilder:rbac:groups=authentication.liqo.io,resources=identities,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=identities/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=identities/finalizers,verbs=update
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=renews,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=renews/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=renews/finalizers,verbs=update

// Reconcile ensures a permanent Renew object exists for the Identity, and syncs
// renewed AuthParams from the Renew status back to the Identity spec.
// Certificate renewal is driven by the provider (Tenant controller); the consumer
// only maintains the Renew as a sync channel.
func (r *LocalRenewerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var identity authv1beta1.Identity
	if err := r.Get(ctx, req.NamespacedName, &identity); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip reconciliation if the Identity is being deleted.
	if !identity.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Ensure a permanent Renew object exists for this Identity.
	if err := r.ensureRenew(ctx, &identity); err != nil {
		klog.Errorf("Unable to ensure Renew for Identity %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Check if there's renewed AuthParams and update the Identity accordingly.
	if err := r.handleRenewedAuthParams(ctx, &identity); err != nil {
		klog.Errorf("Unable to handle renewed AuthParams for Identity %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LocalRenewerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Only reconcile ControlPlane identities.
	controlPlaneFilter := predicate.NewPredicateFuncs(func(object client.Object) bool {
		identity, ok := object.(*authv1beta1.Identity)
		if !ok {
			return false
		}
		return identity.Spec.Type == authv1beta1.ControlPlaneIdentityType
	})

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlRenewLocal).
		Owns(&authv1beta1.Renew{}).
		For(&authv1beta1.Identity{}, builder.WithPredicates(controlPlaneFilter)).
		Complete(r)
}

// ensureRenew ensures a permanent Renew object exists for the given Identity.
// The Renew is created once and never deleted; it serves as a sync channel
// between consumer and provider clusters via the CRD replicator.
func (r *LocalRenewerReconciler) ensureRenew(ctx context.Context, identity *authv1beta1.Identity) error {
	renew := &authv1beta1.Renew{
		ObjectMeta: metav1.ObjectMeta{
			Name:      identity.Name,
			Namespace: identity.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, renew, func() error {
		if renew.Labels == nil {
			renew.Labels = make(map[string]string)
		}
		renew.Labels[consts.ReplicationRequestedLabel] = consts.ReplicationRequestedLabelValue
		renew.Labels[consts.ReplicationDestinationLabel] = string(identity.Spec.ClusterID)
		renew.Labels[consts.RemoteClusterID] = string(identity.Spec.ClusterID)

		if err := controllerutil.SetControllerReference(identity, renew, r.Scheme); err != nil {
			return err
		}

		renew.Spec.ConsumerClusterID = r.LocalClusterID
		renew.Spec.IdentityType = identity.Spec.Type

		return nil
	})

	return err
}

// handleRenewedAuthParams checks if the Renew object has newer AuthParams than the Identity.
// If found, it updates the Identity's Spec.AuthParams with the renewed values.
func (r *LocalRenewerReconciler) handleRenewedAuthParams(ctx context.Context, identity *authv1beta1.Identity) error {
	var renew authv1beta1.Renew
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: identity.Namespace,
		Name:      identity.Name,
	}, &renew); err != nil {
		return client.IgnoreNotFound(err)
	}

	// If the Renew is being deleted or has no AuthParams yet, skip.
	if renew.DeletionTimestamp != nil || renew.Status.AuthParams == nil {
		return nil
	}

	// If the certificate is already up to date, skip.
	if bytes.Equal(renew.Status.AuthParams.SignedCRT, identity.Spec.AuthParams.SignedCRT) {
		return nil
	}

	// Update the Identity's AuthParams with the renewed values.
	identity.Spec.AuthParams = *renew.Status.AuthParams

	if err := r.Update(ctx, identity); err != nil {
		events.EventWithOptions(r.recorder, identity, fmt.Sprintf("Failed to update AuthParams: %s", err),
			&events.Option{EventType: events.Error, Reason: "AuthParamsUpdateFailed"})
		return fmt.Errorf("failed to update Identity AuthParams: %w", err)
	}

	klog.V(4).Infof("Updated AuthParams for Identity %s/%s from completed Renew", identity.Namespace, identity.Name)
	events.Event(r.recorder, identity, "Updated AuthParams from completed certificate renewal")

	return nil
}
