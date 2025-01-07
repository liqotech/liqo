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

package identitycontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	klog "k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// NewIdentityReconciler returns a new IdentityReconciler.
func NewIdentityReconciler(cl client.Client, s *runtime.Scheme, recorder record.EventRecorder, liqoNamespace string) *IdentityReconciler {
	return &IdentityReconciler{
		Client: cl,
		Scheme: s,

		eventRecorder: recorder,

		liqoNamespace: liqoNamespace,
	}
}

// IdentityReconciler reconciles an Identity object.
type IdentityReconciler struct {
	client.Client
	*runtime.Scheme

	eventRecorder record.EventRecorder

	liqoNamespace string
}

// cluster-role
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=identities;identities/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile Identitiy resources and ensure the secret containing the associated kubeconfig.
func (r *IdentityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var identity authv1beta1.Identity
	if err := r.Get(ctx, req.NamespacedName, &identity); err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("identity %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("unable to get identity %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Ensure the kubeconfig secret.
	secret, err := r.ensureKubeconfigSecret(ctx, &identity)
	if err != nil {
		klog.Errorf("unable to ensure kubeconfig secret for identity %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Update the identity status to reference the kubeconfig secret.
	r.handleIdentityStatus(&identity, secret.Name)
	if err := r.Status().Update(ctx, &identity); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlIdentity).
		For(&authv1beta1.Identity{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func (r *IdentityReconciler) ensureKubeconfigSecret(ctx context.Context, identity *authv1beta1.Identity) (*corev1.Secret, error) {
	// Get the private Key encoded in PEM format.
	privateKey, _, err := authentication.GetClusterKeysPEM(ctx, r.Client, r.liqoNamespace)
	if err != nil {
		return nil, fmt.Errorf("unable to get cluster keys: %w", err)
	}

	// If it's an identity used for the CRD Replicator (type: ControlPlane) we set the remote namespace as default
	// for the kubeconfig.
	var namespace *string
	if identity.Spec.Type == authv1beta1.ControlPlaneIdentityType {
		namespace = identity.Spec.Namespace
	}

	// Create or update the secret containing the kubeconfig.
	kubeconfigSecret := forge.KubeconfigSecret(identity)
	op, err := resource.CreateOrUpdate(ctx, r.Client, kubeconfigSecret, func() error {
		if err := forge.MutateKubeconfigSecret(kubeconfigSecret, identity, privateKey, namespace); err != nil {
			return err
		}
		return controllerutil.SetControllerReference(identity, kubeconfigSecret, r.Scheme)
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create or update kubeconfig secret: %w", err)
	}

	if op != controllerutil.OperationResultNone {
		klog.Infof("kubeconfig secret %q %s", client.ObjectKeyFromObject(kubeconfigSecret), op)
		r.eventRecorder.Event(kubeconfigSecret, "Normal", "KubeconfigEnsured", "Kubeconfig secret ensured")
	}

	return kubeconfigSecret, nil
}

// handleIdentityStatus updates the identity status to reference the kubeconfig secret.
func (r *IdentityReconciler) handleIdentityStatus(identity *authv1beta1.Identity, secretName string) {
	identity.Status.KubeconfigSecretRef = &corev1.LocalObjectReference{
		Name: secretName,
	}
}
