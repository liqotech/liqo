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

package noncesignercontroller

import (
	"context"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/record"
	klog "k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	authgetters "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/getters"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils"
)

// NewNonceSignerReconciler returns a new SecretReconciler.
func NewNonceSignerReconciler(cl client.Client, s *runtime.Scheme,
	recorder record.EventRecorder,
	namespaceManager tenantnamespace.Manager, liqoNamespace string) *NonceSignerReconciler {
	return &NonceSignerReconciler{
		Client: cl,
		Scheme: s,

		eventRecorder: recorder,

		namespaceManager: namespaceManager,
		liqoNamespace:    liqoNamespace,
	}
}

// NonceSignerReconciler reconciles a Secret object.
type NonceSignerReconciler struct {
	client.Client
	*runtime.Scheme

	eventRecorder record.EventRecorder

	namespaceManager tenantnamespace.Manager
	liqoNamespace    string
}

// cluster-role
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;delete;update;patch

// Reconcile secrets of type Nonce and sign them.
func (r *NonceSignerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, req.NamespacedName, &secret); err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("secret %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("unable to get secret %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Check if the secret contains the remoteClusterID label.
	remoteClusterID, ok := utils.GetClusterIDFromLabels(secret.Labels)
	if !ok {
		klog.Infof("RemoteClusterID not found in secret %q", req.NamespacedName)
		r.eventRecorder.Event(&secret, "Warning", "MissingRemoteClusterID", "RemoteClusterID not found in secret")
		return ctrl.Result{}, nil
	}

	// Get the tenant namespace for the remote cluster.
	tenantNamespace, err := r.namespaceManager.GetNamespace(ctx, remoteClusterID)
	if err != nil {
		klog.Errorf("Unable to get tenant namespace for cluster %q: %s", remoteClusterID, err)
		r.eventRecorder.Event(&secret, "Warning", "TenantNamespaceNotFound", "Unable to get tenant namespace")
		return ctrl.Result{}, err
	}

	// Check if the secret is in the tenant namespace.
	if secret.Namespace != tenantNamespace.Name {
		klog.Infof("Secret %q not in tenant namespace %q", req.NamespacedName, tenantNamespace.Name)
		r.eventRecorder.Event(&secret, "Warning", "WrongNamespace", "Secret not in tenant namespace")
		return ctrl.Result{}, nil
	}

	// Get cluster keys from the secret.
	privateKey, _, err := authentication.GetClusterKeys(ctx, r.Client, r.liqoNamespace)
	if err != nil {
		klog.Errorf("unable to get cluster keys: %v", err)
		return ctrl.Result{}, err
	}

	// Extract the nonce from the secret.
	nonce, err := authgetters.GetNonceFromSecret(&secret)
	if err != nil {
		klog.Errorf("unable to get nonce from secret %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Sign the nonce using the private key.
	signedNonce := authentication.SignNonce(privateKey, nonce)

	// Check if the secret is already signed and the signature is the same.
	existingSignedNonce, found := secret.Data[consts.SignedNonceSecretField]
	if found && slices.Equal(existingSignedNonce, signedNonce) {
		klog.V(4).Infof("secret %q already signed", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Update the secret with the signed nonce.
	secret.Data[consts.SignedNonceSecretField] = signedNonce
	if err := r.Update(ctx, &secret); err != nil {
		klog.Errorf("unable to update secret: %v", err)
		return ctrl.Result{}, err
	}
	r.eventRecorder.Event(&secret, "Normal", "NonceSigned", "Nonce signed")
	klog.Infof("secret %q signed", req.NamespacedName)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NonceSignerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	req1, err := labels.NewRequirement(consts.SignedNonceSecretLabelKey, selection.Exists, nil)
	if err != nil {
		return err
	}
	selector := labels.NewSelector().Add(*req1)
	filter := predicate.NewPredicateFuncs(func(o client.Object) bool {
		return selector.Matches(labels.Set(o.GetLabels()))
	})

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlSecretNonceSigner).
		For(&corev1.Secret{}, builder.WithPredicates(filter)).
		Complete(r)
}
