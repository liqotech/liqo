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

package wireguard

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/forge"
	enutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/utils"
	"github.com/liqotech/liqo/pkg/utils"
	mapsutil "github.com/liqotech/liqo/pkg/utils/maps"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// WgGatewayServerReconciler manage WgGatewayServer lifecycle.
type WgGatewayServerReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	clusterRoleName string

	eventRecorder record.EventRecorder
}

// NewWgGatewayServerReconciler returns a new WgGatewayServerReconciler.
func NewWgGatewayServerReconciler(cl client.Client, s *runtime.Scheme,
	recorder record.EventRecorder,
	clusterRoleName string) *WgGatewayServerReconciler {
	return &WgGatewayServerReconciler{
		Client:          cl,
		Scheme:          s,
		clusterRoleName: clusterRoleName,

		eventRecorder: recorder,
	}
}

// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservers,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;delete;update
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;delete;create;update;patch
// +kubectl:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;delete;create;update;patch

// Reconcile manage WgGatewayServer lifecycle.
func (r *WgGatewayServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	wgServer := &networkingv1beta1.WgGatewayServer{}
	if err = r.Get(ctx, req.NamespacedName, wgServer); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("WireGuard gateway server %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the WireGuard gateway server %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if !wgServer.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(wgServer, consts.ClusterRoleBindingFinalizer) {
			if err = enutils.DeleteClusterRoleBinding(ctx, r.Client, wgServer); err != nil {
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(wgServer, consts.ClusterRoleBindingFinalizer)
			if err = r.Update(ctx, wgServer); err != nil {
				klog.Errorf("Unable to remove finalizer %q from WireGuard gateway server %q: %v",
					consts.ClusterRoleBindingFinalizer, req.NamespacedName, err)
				return ctrl.Result{}, err
			}
		}

		// Resource is deleting and child resources are deleted as well by garbage collector. Nothing to do.
		return ctrl.Result{}, nil
	}

	originalWgServer := wgServer.DeepCopy()

	// Ensure ServiceAccount and ClusterRoleBinding (create or update)
	if err = enutils.EnsureServiceAccountAndClusterRoleBinding(ctx, r.Client, r.Scheme, &wgServer.Spec.Deployment, wgServer,
		r.clusterRoleName); err != nil {
		return ctrl.Result{}, err
	}

	// update if the wgServer has been updated
	if !equality.Semantic.DeepEqual(originalWgServer, wgServer) {
		if err := r.Update(ctx, wgServer); err != nil {
			return ctrl.Result{}, err
		}

		// we return here to avoid conflicts
		return ctrl.Result{}, nil
	}

	deployNsName := types.NamespacedName{Namespace: wgServer.Namespace, Name: forge.GatewayResourceName(wgServer.Name)}
	svcNsName := types.NamespacedName{Namespace: wgServer.Namespace, Name: forge.GatewayResourceName(wgServer.Name)}

	var deploy *appsv1.Deployment
	var d appsv1.Deployment
	err = r.Get(ctx, deployNsName, &d)
	switch {
	case apierrors.IsNotFound(err):
		deploy = nil
	case err != nil:
		klog.Errorf("Unable to get the deployment %q: %v", deployNsName, err)
		return ctrl.Result{}, err
	default:
		deploy = &d
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
			return
		}

		r.eventRecorder.Event(wgServer, corev1.EventTypeNormal, "Reconciled", "WireGuard gateway server reconciled")
	}()

	if err := r.handleEndpointStatus(ctx, wgServer, svcNsName, deploy); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.handleSecretRefStatus(ctx, wgServer); err != nil {
		klog.Errorf("Error while handling secret ref status: %v", err)
		r.eventRecorder.Event(wgServer, corev1.EventTypeWarning, "SecretRefStatusFailed",
			fmt.Sprintf("Failed to handle secret ref status: %s", err))
		return ctrl.Result{}, err
	}

	if err := r.handleInternalEndpointStatus(ctx, wgServer, svcNsName, deploy); err != nil {
		klog.Errorf("Error while handling internal endpoint status: %v", err)
		r.eventRecorder.Event(wgServer, corev1.EventTypeWarning, "InternalEndpointStatusFailed",
			fmt.Sprintf("Failed to handle internal endpoint status: %s", err))
		return ctrl.Result{}, err
	}

	if wgServer.Spec.SecretRef.Name == "" {
		// Ensure WireGuard keys secret (create or update)
		if err = ensureKeysSecret(ctx, r.Client, wgServer, gateway.ModeServer); err != nil {
			r.eventRecorder.Event(wgServer, corev1.EventTypeWarning, "KeysSecretEnforcedFailed", "Failed to enforce keys secret")
			return ctrl.Result{}, err
		}
		r.eventRecorder.Event(wgServer, corev1.EventTypeNormal, "KeysSecretEnforced", "Enforced keys secret")
	} else {
		// Check that the secret exists and is correctly labeled
		if err = checkExistingKeysSecret(ctx, r.Client, wgServer.Status.SecretRef.Name, wgServer.Namespace); err != nil {
			r.eventRecorder.Event(wgServer, corev1.EventTypeWarning, "KeysSecretCheckFailed", fmt.Sprintf("Failed to check keys secret: %s", err))
			return ctrl.Result{}, err
		}
		r.eventRecorder.Event(wgServer, corev1.EventTypeNormal, "KeysSecretChecked", "Checked keys secret")
	}

	// Ensure deployment (create or update)
	_, err = r.ensureDeployment(ctx, wgServer, deployNsName)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.eventRecorder.Event(wgServer, corev1.EventTypeNormal, "DeploymentEnforced", "Enforced deployment")

	// Ensure service (create or update)
	_, err = r.ensureService(ctx, wgServer, svcNsName)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.eventRecorder.Event(wgServer, corev1.EventTypeNormal, "ServiceEnforced", "Enforced service")

	// Ensure Metrics (if set)
	err = enutils.EnsureMetrics(ctx,
		r.Client, r.Scheme,
		wgServer.Spec.Metrics, wgServer)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.eventRecorder.Event(wgServer, corev1.EventTypeNormal, "MetricsEnforced", "Enforced metrics")

	return ctrl.Result{}, nil
}

// SetupWithManager register the WgGatewayServerReconciler to the manager.
func (r *WgGatewayServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlWGGatewayServer).
		For(&networkingv1beta1.WgGatewayServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(podEnquerer)).
		Watches(&rbacv1.ClusterRoleBinding{},
			handler.EnqueueRequestsFromMapFunc(clusterRoleBindingEnquerer)).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(wireGuardSecretEnquerer),
			builder.WithPredicates(filterWireGuardSecretsPredicate())).
		Complete(r)
}

func (r *WgGatewayServerReconciler) ensureDeployment(ctx context.Context, wgServer *networkingv1beta1.WgGatewayServer,
	depNsName types.NamespacedName) (*appsv1.Deployment, error) {
	dep := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      depNsName.Name,
		Namespace: depNsName.Namespace,
	}}

	op, err := resource.CreateOrUpdate(ctx, r.Client, &dep, func() error {
		return r.mutateFnWgServerDeployment(&dep, wgServer)
	})
	if err != nil {
		klog.Errorf("error while creating/updating deployment %q (operation: %s): %v", depNsName, op, err)
		return nil, err
	}

	klog.Infof("Deployment %q correctly enforced (operation: %s)", depNsName, op)
	return &dep, nil
}

func (r *WgGatewayServerReconciler) ensureService(ctx context.Context, wgServer *networkingv1beta1.WgGatewayServer,
	svcNsName types.NamespacedName) (*corev1.Service, error) {
	svc := corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      svcNsName.Name,
		Namespace: svcNsName.Namespace,
	}}

	op, err := resource.CreateOrUpdate(ctx, r.Client, &svc, func() error {
		return r.mutateFnWgServerService(&svc, wgServer)
	})
	if err != nil {
		klog.Errorf("error while creating/updating service %q (operation: %s): %v", svcNsName, op, err)
		return nil, err
	}

	klog.Infof("Service %q correctly enforced (operation: %s)", svcNsName, op)
	return &svc, nil
}

func (r *WgGatewayServerReconciler) mutateFnWgServerDeployment(deployment *appsv1.Deployment, wgServer *networkingv1beta1.WgGatewayServer) error {
	// Forge metadata
	mapsutil.SmartMergeLabels(deployment, wgServer.Spec.Deployment.Metadata.GetLabels())
	mapsutil.SmartMergeAnnotations(deployment, wgServer.Spec.Deployment.Metadata.GetAnnotations())

	// Forge spec
	deployment.Spec = wgServer.Spec.Deployment.Spec

	if wgServer.Status.SecretRef != nil {
		for i := range deployment.Spec.Template.Spec.Volumes {
			if deployment.Spec.Template.Spec.Volumes[i].Name == wireguardVolumeName {
				deployment.Spec.Template.Spec.Volumes[i].Secret = &corev1.SecretVolumeSource{
					SecretName: wgServer.Status.SecretRef.Name,
				}
				break
			}
		}
	} else {
		r.eventRecorder.Event(wgServer, corev1.EventTypeWarning, "MissingSecretRef", "WireGuard keys secret not found")
	}

	// Set WireGuard server as owner of the deployment
	return controllerutil.SetControllerReference(wgServer, deployment, r.Scheme)
}

func (r *WgGatewayServerReconciler) mutateFnWgServerService(service *corev1.Service, wgServer *networkingv1beta1.WgGatewayServer) error {
	// Forge metadata
	mapsutil.SmartMergeLabels(service, wgServer.Spec.Service.Metadata.GetLabels())
	mapsutil.SmartMergeAnnotations(service, wgServer.Spec.Service.Metadata.GetAnnotations())

	// Forge spec
	serviceClassName := service.Spec.LoadBalancerClass
	service.Spec = wgServer.Spec.Service.Spec
	if wgServer.Spec.Service.Spec.LoadBalancerClass == nil {
		service.Spec.LoadBalancerClass = serviceClassName
	}

	// Set WireGuard server as owner of the service
	return controllerutil.SetControllerReference(wgServer, service, r.Scheme)
}

func (r *WgGatewayServerReconciler) handleEndpointStatus(ctx context.Context, wgServer *networkingv1beta1.WgGatewayServer,
	svcNsName types.NamespacedName, dep *appsv1.Deployment) error {
	if dep == nil {
		wgServer.Status.Endpoint = nil
		return nil
	}

	// Handle WireGuard server Service
	var service corev1.Service
	err := r.Get(ctx, svcNsName, &service)
	if err != nil {
		klog.Error(err) // raise an error also if service NotFound
		return err
	}

	// Put service endpoint in WireGuard server status
	var endpointStatus *networkingv1beta1.EndpointStatus
	switch service.Spec.Type {
	case corev1.ServiceTypeClusterIP:
		endpointStatus, err = r.forgeEndpointStatusClusterIP(&service)
	case corev1.ServiceTypeNodePort:
		endpointStatus, _, err = r.forgeEndpointStatusNodePort(ctx, &service, dep)
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

func (r *WgGatewayServerReconciler) forgeEndpointStatusClusterIP(service *corev1.Service) (*networkingv1beta1.EndpointStatus, error) {
	if len(service.Spec.Ports) == 0 {
		err := fmt.Errorf("service %s/%s has no ports", service.Namespace, service.Name)
		klog.Error(err)
		return nil, err
	}

	port := service.Spec.Ports[0].Port
	protocol := &service.Spec.Ports[0].Protocol
	addresses := service.Spec.ClusterIPs

	return &networkingv1beta1.EndpointStatus{
		Protocol:  protocol,
		Port:      port,
		Addresses: addresses,
	}, nil
}

func (r *WgGatewayServerReconciler) forgeEndpointStatusNodePort(ctx context.Context, service *corev1.Service,
	dep *appsv1.Deployment) (*networkingv1beta1.EndpointStatus, *networkingv1beta1.InternalGatewayEndpoint, error) {
	if len(service.Spec.Ports) == 0 {
		err := fmt.Errorf("service %s/%s has no ports", service.Namespace, service.Name)
		klog.Error(err)
		return nil, nil, err
	}

	port := service.Spec.Ports[0].NodePort
	protocol := &service.Spec.Ports[0].Protocol

	podsSelector := client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(gateway.ForgeActiveGatewayPodLabels())}
	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(dep.Namespace), podsSelector); err != nil {
		klog.Errorf("Unable to list pods of deployment %s/%s: %v", dep.Namespace, dep.Name, err)
		return nil, nil, err
	}

	if len(podList.Items) != 1 {
		err := fmt.Errorf("wrong number of pods for deployment %s/%s: %d (must be 1)", dep.Namespace, dep.Name, len(podList.Items))
		klog.Error(err)
		return nil, nil, err
	}

	pod := &podList.Items[0]

	node := &corev1.Node{}
	err := r.Get(ctx, types.NamespacedName{Name: pod.Spec.NodeName}, node)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("Unable to get node %q: %v", pod.Spec.NodeName, err)
		return nil, nil, err
	}

	addresses := make([]string, 1)
	if utils.IsNodeReady(node) {
		if addresses[0], err = utils.GetAddress(node); err != nil {
			klog.Errorf("Unable to get address of node %q: %v", pod.Spec.NodeName, err)
			return nil, nil, err
		}
	}

	internalAddress := pod.Status.PodIP
	if internalAddress == "" {
		err := fmt.Errorf("pod %s/%s has no IP", pod.Namespace, pod.Name)
		klog.Error(err)
		return nil, nil, err
	}

	return &networkingv1beta1.EndpointStatus{
			Protocol:  protocol,
			Port:      port,
			Addresses: addresses,
		}, &networkingv1beta1.InternalGatewayEndpoint{
			IP:   ptr.To(networkingv1beta1.IP(internalAddress)),
			Node: &pod.Spec.NodeName,
		}, nil
}

func (r *WgGatewayServerReconciler) forgeEndpointStatusLoadBalancer(service *corev1.Service) (*networkingv1beta1.EndpointStatus, error) {
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

	return &networkingv1beta1.EndpointStatus{
		Protocol:  protocol,
		Port:      port,
		Addresses: addresses,
	}, nil
}

func (r *WgGatewayServerReconciler) handleSecretRefStatus(ctx context.Context, wgServer *networkingv1beta1.WgGatewayServer) error {
	secret, err := getWireGuardSecret(ctx, r.Client, wgServer)
	switch {
	case apierrors.IsNotFound(err):
		wgServer.Status.SecretRef = nil
		return nil
	case err != nil:
		return err
	default:
		wgServer.Status.SecretRef = &corev1.ObjectReference{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		}
		return nil
	}
}

func (r *WgGatewayServerReconciler) handleInternalEndpointStatus(ctx context.Context, wgServer *networkingv1beta1.WgGatewayServer,
	svcNsName types.NamespacedName, dep *appsv1.Deployment) error {
	if dep == nil {
		wgServer.Status.InternalEndpoint = nil
		return nil
	}

	var service corev1.Service
	err := r.Get(ctx, svcNsName, &service)
	if err != nil {
		klog.Error(err) // raise an error also if service NotFound
		return err
	}

	_, ige, err := r.forgeEndpointStatusNodePort(ctx, &service, dep)
	if err != nil {
		return err
	}

	wgServer.Status.InternalEndpoint = ige
	return nil
}
