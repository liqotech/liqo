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

package noncecreatorcontroller

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/liqotech/liqo/pkg/consts"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils"
)

// NonceCreatorReconciler manage Nonces lifecycle.
type NonceCreatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	NamespaceManager tenantnamespace.Manager
	EventRecorder    record.EventRecorder
}

// NewNonceReconciler returns a new NonceReconciler.
func NewNonceReconciler(cl client.Client, s *runtime.Scheme,
	namespaceManager tenantnamespace.Manager,
	recorder record.EventRecorder) *NonceCreatorReconciler {
	return &NonceCreatorReconciler{
		Client: cl,
		Scheme: s,

		NamespaceManager: namespaceManager,
		EventRecorder:    recorder,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;delete;create;update;patch

// Reconcile manage Nonce lifecycle.
func (r *NonceCreatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	secret := &corev1.Secret{}
	if err = r.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Secret %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the secret %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if _, ok := secret.Data[consts.NonceSecretField]; ok {
		klog.V(4).Infof("Nonce already found in secret %q", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	if secret.Labels == nil {
		klog.Infof("Labels not found in secret %q", req.NamespacedName)
		r.EventRecorder.Event(secret, "Warning", "MissingLabels", "Labels not found in secret")
		return ctrl.Result{}, nil
	}

	clusterID, ok := utils.GetClusterIDFromLabels(secret.Labels)
	if !ok {
		klog.Infof("ClusterID not found in secret %q", req.NamespacedName)
		r.EventRecorder.Event(secret, "Warning", "MissingClusterID", "ClusterID not found in secret")
		return ctrl.Result{}, nil
	}

	tenantNamespace, err := r.NamespaceManager.GetNamespace(ctx, clusterID)
	if err != nil {
		klog.Errorf("Unable to get tenant namespace for cluster %q: %s", clusterID, err)
		r.EventRecorder.Event(secret, "Warning", "TenantNamespaceNotFound", "Unable to get tenant namespace")
		return ctrl.Result{}, err
	}

	if secret.Namespace != tenantNamespace.Name {
		klog.Infof("Secret %q not in tenant namespace %q", req.NamespacedName, tenantNamespace.Name)
		r.EventRecorder.Event(secret, "Warning", "WrongNamespace", "Secret not in tenant namespace")
		return ctrl.Result{}, nil
	}

	nonce, err := generateNonce()
	if err != nil {
		klog.Errorf("Unable to generate nonce: %s", err)
		r.EventRecorder.Event(secret, "Warning", "NonceGenerationFailed", "Unable to generate nonce")
		return ctrl.Result{}, err
	}

	if secret.StringData == nil {
		secret.StringData = make(map[string]string)
	}

	secret.StringData[consts.NonceSecretField] = nonce

	if err = r.Update(ctx, secret); err != nil {
		klog.Errorf("Unable to update secret %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	klog.Infof("Nonce generated for secret %q and cluster %q", req.NamespacedName, clusterID)
	r.EventRecorder.Event(secret, "Normal", "NonceGenerated", "Nonce generated")
	return ctrl.Result{}, nil
}

func generateNonce() (string, error) {
	nonce := make([]byte, 64)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(nonce), nil
}

// SetupWithManager register the NonceReconciler with the manager.
func (r *NonceCreatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	req1, err := labels.NewRequirement(consts.NonceSecretLabelKey, selection.Exists, nil)
	if err != nil {
		return err
	}

	selector := labels.NewSelector().Add(*req1)
	filter := predicate.NewPredicateFuncs(func(o client.Object) bool {
		return selector.Matches(labels.Set(o.GetLabels()))
	})

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlSecretNonceCreator).
		For(&corev1.Secret{}, builder.WithPredicates(filter)).
		Complete(r)
}
