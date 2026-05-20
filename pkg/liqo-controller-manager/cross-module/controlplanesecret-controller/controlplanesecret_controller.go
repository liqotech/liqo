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

package controlplanesecretcontroller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqocorev1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/consts"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// ControlPlaneSecretReconciler creates namespacemaps from controlplane secrets.
type ControlPlaneSecretReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	EventRecorder events.EventRecorder
}

// NewControlPlaneSecretReconciler returns a new ControlPlaneSecretReconciler.
func NewControlPlaneSecretReconciler(cl client.Client, s *runtime.Scheme, recorder events.EventRecorder) *ControlPlaneSecretReconciler {
	return &ControlPlaneSecretReconciler{
		Client: cl,
		Scheme: s,

		EventRecorder: recorder,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespacemaps,verbs=get;watch;list;update;patch;create;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile controlplane secrets and create their associated namespacemaps.
func (r *ControlPlaneSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := klog.FromContext(ctx).WithValues("secret", req.NamespacedName)

	var secret corev1.Secret
	if err := r.Get(ctx, req.NamespacedName, &secret); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, fmt.Errorf("getting secret %q: %w", req.NamespacedName, err)
		}
		return ctrl.Result{}, nil
	}

	remoteClusterID := liqocorev1beta1.ClusterID(secret.Labels[consts.RemoteClusterID])
	if remoteClusterID == "" {
		// This should not happen, as the controller is watching only secrets with the RemoteClusterID label,
		// but we check it anyway for extra safety in case of empty value. We can return without doing anything.
		logger.Info("Controlplane secret does not have the RemoteClusterID label, skipping")
		return ctrl.Result{}, nil
	}

	if v, ok := secret.Annotations[consts.SkipNamespaceMapCreationAnnotationKey]; ok &&
		strings.EqualFold(v, consts.SkipNamespaceMapCreationAnnotationValue) {
		logger.V(6).Info("NamespaceMap creation disabled for controlplane secret")
		return ctrl.Result{}, nil
	}

	if !secret.DeletionTimestamp.IsZero() {
		// Although NamespaceMap has ownerReference to the secret, we have to delete NamespaceMap anyway since secret has
		// a finalizer which prevent deletion, which will not be deleted by the CRD replicator until all replicated resources
		// (including the NamespaceMap) are deleted. So we cannot avoid cascading deletion as this NamespaceMap will never be
		// marked for deletion by Kubernetes garbage collector until the secret is actually deleted.
		if err := r.deleteNamespaceMap(ctx, &secret, remoteClusterID); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.ensureNamespaceMap(ctx, &secret, remoteClusterID); err != nil {
		return ctrl.Result{}, fmt.Errorf("ensuring NamespaceMap from %s secret (remoteClusterID: %s): %w", req.NamespacedName, remoteClusterID, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the ControlPlaneSecretReconciler with the manager.
func (r *ControlPlaneSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	cpSecretFilter, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      consts.RemoteClusterID,
				Operator: metav1.LabelSelectorOpExists,
			},
			{
				Key:      consts.IdentityTypeLabelKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{string(authv1beta1.ControlPlaneIdentityType)},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("creating controlplane secret predicate: %w", err)
	}

	// generate the predicate to filter just the ResourceSlices created by the remote cluster checking crdReplicator labels
	localNsMapFilter, err := predicate.LabelSelectorPredicate(reflection.LocalResourcesLabelSelector())
	if err != nil {
		return fmt.Errorf("creating local NamespaceMap predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlSecretNsMapCreator).
		For(&corev1.Secret{}, builder.WithPredicates(cpSecretFilter)).
		Owns(&offloadingv1beta1.NamespaceMap{}, builder.WithPredicates(localNsMapFilter)).
		Complete(r)
}

// deleteNamespaceMap deletes the NamespaceMap associated with the controlplane secret.
func (r *ControlPlaneSecretReconciler) deleteNamespaceMap(ctx context.Context,
	cpSecret *corev1.Secret, remoteClusterID liqocorev1beta1.ClusterID) error {
	nm := offloadingv1beta1.NamespaceMap{ObjectMeta: metav1.ObjectMeta{
		Name:      fcutils.UniqueName(remoteClusterID),
		Namespace: cpSecret.Namespace,
	}}
	if err := client.IgnoreNotFound(r.Delete(ctx, &nm)); err != nil {
		return fmt.Errorf("deleting NamespaceMap %s: %w", client.ObjectKeyFromObject(&nm), err)
	}
	return nil
}

// ensureNamespaceMap ensures the presence of a NamespaceMap associated with the controlplane secret.
func (r *ControlPlaneSecretReconciler) ensureNamespaceMap(ctx context.Context,
	cpSecret *corev1.Secret, remoteClusterID liqocorev1beta1.ClusterID) error {
	logger := klog.FromContext(ctx).WithValues("remoteClusterID", remoteClusterID)

	nm := offloadingv1beta1.NamespaceMap{ObjectMeta: metav1.ObjectMeta{
		Name:      fcutils.UniqueName(remoteClusterID),
		Namespace: cpSecret.Namespace,
	}}
	op, err := resource.CreateOrUpdate(ctx, r.Client, &nm, func() error {
		if nm.Labels == nil {
			nm.Labels = map[string]string{}
		}
		nm.Labels[consts.RemoteClusterID] = string(remoteClusterID)
		nm.Labels[consts.ReplicationRequestedLabel] = consts.ReplicationRequestedLabelValue
		nm.Labels[consts.ReplicationDestinationLabel] = string(remoteClusterID)

		// Set owner reference to the controlplane secret, to automatically delete the NamespaceMap when the secret is deleted.
		return controllerutil.SetControllerReference(cpSecret, &nm, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("creating or updating NamespaceMap %s: %w", client.ObjectKeyFromObject(&nm), err)
	}
	if op != controllerutil.OperationResultNone {
		logger.Info("NamespaceMap successfully enforced", "operation", op)
	}

	r.EventRecorder.Eventf(cpSecret, &nm, corev1.EventTypeNormal, "NamespaceMapEnsured", string(op),
		"NamespaceMap %s created from ControlPlane Secret", client.ObjectKeyFromObject(&nm))

	return nil
}
