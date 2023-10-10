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

package wireguard

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	enutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/utils"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
	mapsutil "github.com/liqotech/liqo/pkg/utils/maps"
)

// WgGatewayClientReconciler manage WgGatewayClient lifecycle.
type WgGatewayClientReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	clusterRoleName string
}

// NewWgGatewayClientReconciler returns a new WgGatewayClientReconciler.
func NewWgGatewayClientReconciler(cl client.Client, s *runtime.Scheme, clusterRoleName string) *WgGatewayClientReconciler {
	return &WgGatewayClientReconciler{
		Client:          cl,
		Scheme:          s,
		clusterRoleName: clusterRoleName,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayclients,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayclients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;delete;create;update;patch
// +kubectl:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;delete;create;update;patch

// Reconcile manage GatewayClient lifecycle.
func (r *WgGatewayClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	wgClient := &networkingv1alpha1.WgGatewayClient{}
	if err = r.Get(ctx, req.NamespacedName, wgClient); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("WireGuard gateway client %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the WireGuard gateway client %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if !wgClient.DeletionTimestamp.IsZero() {
		// Resource is deleting and child resources are deleted as well by garbage collector. Nothing to do.
		return ctrl.Result{}, nil
	}

	// Ensure ServiceAccount and RoleBinding (create or update)
	if err = enutils.EnsureServiceAccountAndRoleBinding(ctx, r.Client, r.Scheme, &wgClient.Spec.Deployment, wgClient,
		r.clusterRoleName); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure deployment (create or update)
	deployNsName := types.NamespacedName{Namespace: wgClient.Namespace, Name: wgClient.Name}
	_, err = r.ensureDeployment(ctx, wgClient, deployNsName)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure Metrics (if set)
	err = enutils.EnsureMetrics(ctx,
		r.Client, r.Scheme,
		wgClient.Spec.Metrics, wgClient)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Handle status
	defer func() {
		newErr := r.Status().Update(ctx, wgClient)
		if newErr != nil {
			if err != nil {
				klog.Errorf("Error reconciling the WireGuard gateway client %q: %s", req.NamespacedName, err)
			}
			klog.Errorf("Unable to update the WireGuard gateway client status %q: %s", req.NamespacedName, newErr)
			err = newErr
		}
	}()

	if err := r.handleSecretRefStatus(ctx, wgClient); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the WgGatewayClientReconciler to the manager.
func (r *WgGatewayClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.WgGatewayClient{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.RoleBinding{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.secretEnquerer), builder.WithPredicates(r.filterSecretsPredicate())).
		Complete(r)
}

func (r *WgGatewayClientReconciler) filterSecretsPredicate() predicate.Predicate {
	filterWgClientSecrets, err := predicate.LabelSelectorPredicate(liqolabels.WgClientNameLabelSelector)
	utilruntime.Must(err)
	return filterWgClientSecrets
}

func (r *WgGatewayClientReconciler) secretEnquerer(_ context.Context, obj client.Object) []ctrl.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	wgClientName, found := secret.GetLabels()[consts.WgClientNameLabel]
	if !found {
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: secret.Namespace,
				Name:      wgClientName,
			},
		},
	}
}

func (r *WgGatewayClientReconciler) ensureDeployment(ctx context.Context, wgClient *networkingv1alpha1.WgGatewayClient,
	depNsName types.NamespacedName) (*appsv1.Deployment, error) {
	dep := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      depNsName.Name,
		Namespace: depNsName.Namespace,
	}}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &dep, func() error {
		return r.mutateFnWgClientDeployment(&dep, wgClient)
	})
	if err != nil {
		klog.Errorf("error while creating/updating deployment %q (operation: %s): %v", depNsName, op, err)
		return nil, err
	}

	klog.Infof("Deployment %q correctly enforced (operation: %s)", depNsName, op)
	return &dep, nil
}

func (r *WgGatewayClientReconciler) mutateFnWgClientDeployment(deployment *appsv1.Deployment, wgClient *networkingv1alpha1.WgGatewayClient) error {
	// Forge metadata
	mapsutil.SmartMergeLabels(deployment, wgClient.Spec.Deployment.Metadata.GetLabels())
	mapsutil.SmartMergeAnnotations(deployment, wgClient.Spec.Deployment.Metadata.GetAnnotations())

	// Forge spec
	deployment.Spec = wgClient.Spec.Deployment.Spec

	if deployment.Spec.Template.ObjectMeta.Labels == nil {
		deployment.Spec.Template.ObjectMeta.Labels = map[string]string{}
	}
	deployment.Spec.Template.ObjectMeta.Labels[consts.ExternalNetworkLabel] = consts.ExternalNetworkLabelValue

	// Set WireGuard client as owner of the deployment
	return controllerutil.SetControllerReference(wgClient, deployment, r.Scheme)
}

func (r *WgGatewayClientReconciler) handleSecretRefStatus(ctx context.Context, wgClient *networkingv1alpha1.WgGatewayClient) error {
	secret, err := r.getWgClientKeysSecret(ctx, wgClient)
	if err != nil {
		return err
	}

	// Put secret reference in WireGuard client status
	if secret == nil {
		// if the secret is not found, we cancel the reference as it could be not valid anymore
		wgClient.Status.SecretRef = nil
	} else {
		wgClient.Status.SecretRef = &corev1.ObjectReference{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		}
	}

	return nil
}

func (r *WgGatewayClientReconciler) getWgClientKeysSecret(ctx context.Context, wgClient *networkingv1alpha1.WgGatewayClient) (*corev1.Secret, error) {
	wgClientSelector := client.MatchingLabels{
		consts.WgClientNameLabel: wgClient.Name, // secret created by the WireGuard client with the given name
	}

	var secrets corev1.SecretList
	err := r.List(ctx, &secrets, client.InNamespace(wgClient.Namespace), wgClientSelector)
	if err != nil {
		klog.Errorf("Unable to list secrets associated to WireGuard client %s/%s: %v", wgClient.Namespace, wgClient.Name, err)
		return nil, err
	}

	switch len(secrets.Items) {
	case 0:
		klog.Warningf("Secret associated to WireGuard client %s/%s not found", wgClient.Namespace, wgClient.Name)
		return nil, nil
	case 1:
		return &secrets.Items[0], nil
	default:
		return nil, fmt.Errorf("found multiple secrets associated to WireGuard client %s/%s", wgClient.Namespace, wgClient.Name)
	}
}
