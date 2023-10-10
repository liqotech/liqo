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

	"golang.org/x/exp/maps"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	"github.com/liqotech/liqo/pkg/discovery"
	enutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/utils"
	"github.com/liqotech/liqo/pkg/utils"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
	mapsutil "github.com/liqotech/liqo/pkg/utils/maps"
)

// WgGatewayServerReconciler manage WgGatewayServer lifecycle.
type WgGatewayServerReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	extNetPodsClient client.Client
	clusterRoleName  string
}

// NewWgGatewayServerReconciler returns a new WgGatewayServerReconciler.
func NewWgGatewayServerReconciler(cl client.Client, s *runtime.Scheme, extNetPodsClient client.Client,
	clusterRoleName string) *WgGatewayServerReconciler {
	return &WgGatewayServerReconciler{
		Client:           cl,
		Scheme:           s,
		extNetPodsClient: extNetPodsClient,
		clusterRoleName:  clusterRoleName,
	}
}

// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservers,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;delete;create;update;patch
// +kubectl:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;delete;create;update;patch

// Reconcile manage GatewayServer lifecycle.
func (r *WgGatewayServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	wgServer := &networkingv1alpha1.WgGatewayServer{}
	if err = r.Get(ctx, req.NamespacedName, wgServer); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("WireGuard gateway server %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the WireGuard gateway server %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if !wgServer.DeletionTimestamp.IsZero() {
		// Resource is deleting and child resources are deleted as well by garbage collector. Nothing to do.
		return ctrl.Result{}, nil
	}

	// Ensure ServiceAccount and RoleBinding (create or update)
	if err = enutils.EnsureServiceAccountAndRoleBinding(ctx, r.Client, r.Scheme, &wgServer.Spec.Deployment, wgServer,
		r.clusterRoleName); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure deployment (create or update)
	deployNsName := types.NamespacedName{Namespace: wgServer.Namespace, Name: wgServer.Name}
	deploy, err := r.ensureDeployment(ctx, wgServer, deployNsName)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure service (create or update)
	svcNsName := types.NamespacedName{Namespace: wgServer.Namespace, Name: wgServer.Name}
	_, err = r.ensureService(ctx, wgServer, svcNsName)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure Metrics (if set)
	err = enutils.EnsureMetrics(ctx,
		r.Client, r.Scheme,
		wgServer.Spec.Metrics, wgServer)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Handle status
	defer func() {
		newErr := r.Status().Update(ctx, wgServer)
		if newErr != nil {
			if err != nil {
				klog.Errorf("Error reconciling the WireGuard gateway server %q: %s", req.NamespacedName, err)
			}
			klog.Errorf("Unable to update the WireGuard gateway server status %q: %s", req.NamespacedName, newErr)
			err = newErr
		}
	}()

	if err := r.handleEndpointStatus(ctx, wgServer, svcNsName, deploy); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.handleSecretRefStatus(ctx, wgServer); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the WgGatewayServerReconciler to the manager.
func (r *WgGatewayServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.WgGatewayServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.RoleBinding{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.secretEnquerer), builder.WithPredicates(r.filterSecretsPredicate())).
		Complete(r)
}

func (r *WgGatewayServerReconciler) filterSecretsPredicate() predicate.Predicate {
	filterWgServerSecrets, err := predicate.LabelSelectorPredicate(liqolabels.WgServerNameLabelSelector)
	utilruntime.Must(err)
	return filterWgServerSecrets
}

func (r *WgGatewayServerReconciler) secretEnquerer(_ context.Context, obj client.Object) []ctrl.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	wgServerName, found := secret.GetLabels()[consts.WgServerNameLabel]
	if !found {
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: secret.Namespace,
				Name:      wgServerName,
			},
		},
	}
}

func (r *WgGatewayServerReconciler) ensureDeployment(ctx context.Context, wgServer *networkingv1alpha1.WgGatewayServer,
	depNsName types.NamespacedName) (*appsv1.Deployment, error) {
	dep := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      depNsName.Name,
		Namespace: depNsName.Namespace,
	}}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &dep, func() error {
		return r.mutateFnWgServerDeployment(&dep, wgServer)
	})
	if err != nil {
		klog.Errorf("error while creating/updating deployment %q (operation: %s): %v", depNsName, op, err)
		return nil, err
	}

	klog.Infof("Deployment %q correctly enforced (operation: %s)", depNsName, op)
	return &dep, nil
}

func (r *WgGatewayServerReconciler) ensureService(ctx context.Context, wgServer *networkingv1alpha1.WgGatewayServer,
	svcNsName types.NamespacedName) (*corev1.Service, error) {
	svc := corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      svcNsName.Name,
		Namespace: svcNsName.Namespace,
	}}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, &svc, func() error {
		return r.mutateFnWgServerService(&svc, wgServer)
	})
	if err != nil {
		klog.Errorf("error while creating/updating service %q (operation: %s): %v", svcNsName, op, err)
		return nil, err
	}

	klog.Infof("Service %q correctly enforced (operation: %s)", svcNsName, op)
	return &svc, nil
}

func (r *WgGatewayServerReconciler) mutateFnWgServerDeployment(deployment *appsv1.Deployment, wgServer *networkingv1alpha1.WgGatewayServer) error {
	// Forge metadata
	mapsutil.SmartMergeLabels(deployment, wgServer.Spec.Deployment.Metadata.GetLabels())
	mapsutil.SmartMergeAnnotations(deployment, wgServer.Spec.Deployment.Metadata.GetAnnotations())

	// Forge spec
	deployment.Spec = wgServer.Spec.Deployment.Spec

	if deployment.Spec.Template.ObjectMeta.Labels == nil {
		deployment.Spec.Template.ObjectMeta.Labels = map[string]string{}
	}
	deployment.Spec.Template.ObjectMeta.Labels[consts.ExternalNetworkLabel] = consts.ExternalNetworkLabelValue

	// Set WireGuard server as owner of the deployment
	return controllerutil.SetControllerReference(wgServer, deployment, r.Scheme)
}

func (r *WgGatewayServerReconciler) mutateFnWgServerService(service *corev1.Service, wgServer *networkingv1alpha1.WgGatewayServer) error {
	// Forge metadata
	mapsutil.SmartMergeLabels(service, wgServer.Spec.Service.Metadata.GetLabels())
	mapsutil.SmartMergeAnnotations(service, wgServer.Spec.Service.Metadata.GetAnnotations())

	// Forge spec
	service.Spec = wgServer.Spec.Service.Spec

	// Set WireGuard server as owner of the service
	return controllerutil.SetControllerReference(wgServer, service, r.Scheme)
}

func (r *WgGatewayServerReconciler) handleEndpointStatus(ctx context.Context, wgServer *networkingv1alpha1.WgGatewayServer,
	svcNsName types.NamespacedName, dep *appsv1.Deployment) error {
	// Handle WireGuard server Service
	var service corev1.Service
	err := r.Get(ctx, svcNsName, &service)
	if err != nil {
		klog.Error(err) // raise an error also if service NotFound
		return err
	}

	// Put service endpoint in WireGuard server status
	var endpointStatus *networkingv1alpha1.EndpointStatus
	switch service.Spec.Type {
	case corev1.ServiceTypeNodePort:
		endpointStatus, err = r.forgeEndpointStatusNodePort(ctx, &service, dep)
	case corev1.ServiceTypeLoadBalancer:
		endpointStatus, err = r.forgeEndpointStatusLoadBalancer(&service)
	default:
		err = fmt.Errorf("service type %q not supported for WireGuard server Service %q", service.Spec.Type, svcNsName)
		klog.Error(err)
		wgServer.Status.Endpoint = nil // we empty the endpoint status to avoid misaligned spec and status
	}

	if err != nil {
		return err
	}

	wgServer.Status.Endpoint = endpointStatus

	return nil
}

func (r *WgGatewayServerReconciler) forgeEndpointStatusNodePort(ctx context.Context, service *corev1.Service,
	dep *appsv1.Deployment) (*networkingv1alpha1.EndpointStatus, error) {
	if len(service.Spec.Ports) == 0 {
		err := fmt.Errorf("service %s/%s has no ports", service.Namespace, service.Name)
		klog.Error(err)
		return nil, err
	}

	port := service.Spec.Ports[0].NodePort
	protocol := &service.Spec.Ports[0].Protocol

	// Every node IP is a valid endpoint. For convenience, we get the IP of all nodes hosting replicas of the deployment
	// (i.e., WireGuard gateway servers).
	var addresses []string
	podsFromDepSelector := client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)}
	var podList corev1.PodList
	if err := r.extNetPodsClient.List(ctx, &podList, client.InNamespace(dep.Namespace), podsFromDepSelector); err != nil {
		klog.Errorf("Unable to list pods of deployment %s/%s: %v", dep.Namespace, dep.Name, err)
		return nil, err
	}

	// Check if the number of pods found (i.e., in cache) matches the number of desired replicas.
	if err := r.numPodsMatchesDesiredReplicas(len(podList.Items), dep); err != nil {
		return nil, err
	}

	switch len(podList.Items) {
	case 0:
		err := fmt.Errorf("pods of deployment %s/%s not found", dep.Namespace, dep.Name)
		klog.Error(err)
		return nil, err
	default:
		// TODO: if using active-passive, it should get the IP of the active node
		// Get all nodes hosting pod replicas of the WireGuard server deployment
		var nodes []*corev1.Node
		for i := range podList.Items {
			pod := &podList.Items[i]
			// get node hosting pod
			var node corev1.Node
			err := r.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, &node)
			if err != nil && !apierrors.IsNotFound(err) {
				klog.Errorf("Unable to get node %q: %v", pod.Spec.NodeName, err)
				return nil, err
			}
			if !apierrors.IsNotFound(err) {
				nodes = append(nodes, &node)
			}
		}
		// For every node, get IP address. We avoid duplicate utilizing a map and then converting to array.
		// Note that duplicates should not happen if the deployment correctly have replicas spread across different nodes,
		// but we double check anyway.
		addressesMap := make(map[string]interface{})
		for i := range nodes {
			if utils.IsNodeReady(nodes[i]) {
				address, err := discovery.GetAddress(nodes[i])
				if err == nil {
					addressesMap[address] = nil
				}
			}
		}
		// If addressesMap is empty, it could be due to temporary not ready nodes.
		// In this case, we choose a random one (e.g., the first)
		if len(addressesMap) == 0 {
			if len(nodes) > 0 {
				address, err := discovery.GetAddress(nodes[0])
				if err == nil {
					addressesMap[address] = nil
				}
			}
		}
		// If addressesMap is still empty, we raise an error
		if len(addressesMap) == 0 {
			err := fmt.Errorf("no valid addresses found for WireGuard server %s/%s", service.Namespace, service.Name)
			klog.Error(err)
			return nil, err
		}
		// Addresses contains only the keys to avoid duplicates
		addresses = maps.Keys(addressesMap)
	}

	return &networkingv1alpha1.EndpointStatus{
		Protocol:  protocol,
		Port:      port,
		Addresses: addresses,
	}, nil
}

func (r *WgGatewayServerReconciler) numPodsMatchesDesiredReplicas(numPods int, dep *appsv1.Deployment) error {
	var desiredReplicas int
	if dep.Spec.Replicas != nil {
		desiredReplicas = int(*dep.Spec.Replicas)
	} else {
		desiredReplicas = 1 // default value if field is nil, as specified in the official API
	}

	if numPods != desiredReplicas {
		// The number of pods listed does not match the desired replicas, possibly due to a cache sync error.
		// We raise an error to force requeue.
		err := fmt.Errorf("pods found for deployment %s/%s (%d) does not match desired replicas (%d), possible cache sync error",
			dep.Namespace, dep.Name, numPods, desiredReplicas)
		klog.Warning(err)
		return err
	}

	return nil
}

func (r *WgGatewayServerReconciler) forgeEndpointStatusLoadBalancer(service *corev1.Service) (*networkingv1alpha1.EndpointStatus, error) {
	if len(service.Spec.Ports) == 0 {
		err := fmt.Errorf("service %s/%s has no ports", service.Namespace, service.Name)
		klog.Error(err)
		return nil, err
	}

	port := service.Spec.Ports[0].Port
	protocol := &service.Spec.Ports[0].Protocol

	var addresses []string
	for i := range service.Status.LoadBalancer.Ingress {
		if hostName := service.Status.LoadBalancer.Ingress[i].Hostname; hostName != "" {
			addresses = append(addresses, hostName)
		}
		if ip := service.Status.LoadBalancer.Ingress[i].IP; ip != "" {
			addresses = append(addresses, ip)
		}
	}

	return &networkingv1alpha1.EndpointStatus{
		Protocol:  protocol,
		Port:      port,
		Addresses: addresses,
	}, nil
}

func (r *WgGatewayServerReconciler) handleSecretRefStatus(ctx context.Context, wgServer *networkingv1alpha1.WgGatewayServer) error {
	secret, err := r.getWgServerKeysSecret(ctx, wgServer)
	if err != nil {
		return err
	}

	// Put secret reference in WireGuard server status
	if secret == nil {
		// if the secret is not found, we cancel the reference as it could be not valid anymore
		wgServer.Status.SecretRef = nil
	} else {
		wgServer.Status.SecretRef = &corev1.ObjectReference{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		}
	}

	return nil
}

func (r *WgGatewayServerReconciler) getWgServerKeysSecret(ctx context.Context, wgServer *networkingv1alpha1.WgGatewayServer) (*corev1.Secret, error) {
	wgServerSelector := client.MatchingLabels{
		consts.WgServerNameLabel: wgServer.Name, // secret created by the WireGuard server with the given name
	}

	var secrets corev1.SecretList
	err := r.List(ctx, &secrets, client.InNamespace(wgServer.Namespace), wgServerSelector)
	if err != nil {
		klog.Errorf("Unable to list secrets associated to WireGuard server %s/%s: %v", wgServer.Namespace, wgServer.Name, err)
		return nil, err
	}

	switch len(secrets.Items) {
	case 0:
		klog.Warningf("Secret associated to WireGuard server %s/%s not found", wgServer.Namespace, wgServer.Name)
		return nil, nil
	case 1:
		return &secrets.Items[0], nil
	default:
		return nil, fmt.Errorf("found multiple secrets associated to WireGuard server %s/%s", wgServer.Namespace, wgServer.Name)
	}
}
