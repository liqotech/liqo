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
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway"
	enutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/utils"
	mapsutil "github.com/liqotech/liqo/pkg/utils/maps"
)

// WgGatewayClientReconciler manage WgGatewayClient lifecycle.
type WgGatewayClientReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	extNetPodsClient client.Client
	clusterRoleName  string
}

// NewWgGatewayClientReconciler returns a new WgGatewayClientReconciler.
func NewWgGatewayClientReconciler(cl client.Client, s *runtime.Scheme, extNetPodsClient client.Client,
	clusterRoleName string) *WgGatewayClientReconciler {
	return &WgGatewayClientReconciler{
		Client:           cl,
		Scheme:           s,
		extNetPodsClient: extNetPodsClient,
		clusterRoleName:  clusterRoleName,
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
	var deploy *appsv1.Deployment
	deployNsName := types.NamespacedName{Namespace: wgClient.Namespace, Name: gateway.GenerateResourceName(wgClient.Name)}
	deploy, err = r.ensureDeployment(ctx, wgClient, deployNsName)
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

	if err := r.handleInternalEndpointStatus(ctx, wgClient, deploy); err != nil {
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
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(wireGuardSecretEnquerer),
			builder.WithPredicates(filterWireGuardSecretsPredicate())).
		Complete(r)
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
	secret, err := getWireGuardSecret(ctx, r.Client, wgClient)
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

func (r *WgGatewayClientReconciler) handleInternalEndpointStatus(ctx context.Context,
	wgClient *networkingv1alpha1.WgGatewayClient, dep *appsv1.Deployment) error {
	podsFromDepSelector := client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)}
	var podList corev1.PodList
	if err := r.extNetPodsClient.List(ctx, &podList, client.InNamespace(dep.Namespace), podsFromDepSelector); err != nil {
		klog.Errorf("Unable to list pods of deployment %s/%s: %v", dep.Namespace, dep.Name, err)
		return err
	}

	if len(podList.Items) == 0 {
		err := fmt.Errorf("no pods found for deployment %s/%s", dep.Namespace, dep.Name)
		klog.Error(err)
		return err
	}

	// sort pods by creation timestamp (older first), and name
	sort.Slice(podList.Items, func(i, j int) bool {
		if podList.Items[i].CreationTimestamp.Equal(&podList.Items[j].CreationTimestamp) {
			return podList.Items[i].Name < podList.Items[j].Name
		}
		return podList.Items[i].CreationTimestamp.Before(&podList.Items[j].CreationTimestamp)
	})

	if podList.Items[0].Status.PodIP == "" {
		err := fmt.Errorf("pod %s/%s has no IP", podList.Items[0].Namespace, podList.Items[0].Name)
		klog.Error(err)
		return err
	}

	wgClient.Status.InternalEndpoint = &networkingv1alpha1.InternalGatewayEndpoint{
		IP:   ptr.To(networkingv1alpha1.IP(podList.Items[0].Status.PodIP)),
		Node: &podList.Items[0].Spec.NodeName,
	}
	return nil
}
