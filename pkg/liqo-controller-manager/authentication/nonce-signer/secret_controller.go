// Copyright 2019-2024 The Liqo Authors
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

package noncesigner

import (
	"context"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	klog "k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
)

// NewSecretReconciler returns a new SecretReconciler.
func NewSecretReconciler(mgr ctrl.Manager, liqoNamespace string) *SecretReconciler {
	return &SecretReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		liqoNamespace: liqoNamespace,
	}
}

// SecretReconciler reconciles a Secret object.
type SecretReconciler struct {
	client.Client
	*runtime.Scheme

	liqoNamespace string
}

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;delete;update;patch

// Reconcile secrets of type Nonce and sign them.
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, req.NamespacedName, &secret); err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("secret %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("unable to get secret %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Extract the nonce from the secret.
	nonce, err := GetNonceFromSecret(&secret)
	if err != nil {
		klog.Errorf("unable to get nonce from secret %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Get cluster keys from the secret.
	privateKey, _, err := authentication.GetClusterKeys(ctx, r.Client, r.liqoNamespace)
	if err != nil {
		klog.Errorf("unable to get cluster keys: %v", err)
		return ctrl.Result{}, err
	}

	// Sign the nonce using the private key.
	signedNonce := authentication.SignNonce(privateKey, nonce)

	// Check if the secret is already signed.
	existingSignedNonce, found := secret.Data[consts.SignedNonceSecretField]
	if found && slices.Equal(existingSignedNonce, signedNonce) {
		klog.V(4).Infof("secret %q already signed", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	secret.Data[consts.SignedNonceSecretField] = signedNonce
	if err := r.Update(ctx, &secret); err != nil {
		klog.Errorf("unable to update secret: %v", err)
		return ctrl.Result{}, err
	}
	klog.Infof("secret %q signed", req.NamespacedName)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	filter, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			consts.NonceSecretLabelKey: consts.NonceSecretConsumerLabelValue,
		},
	})
	if err != nil {
		klog.Error(err)
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}, builder.WithPredicates(filter)).
		Complete(r)
}
