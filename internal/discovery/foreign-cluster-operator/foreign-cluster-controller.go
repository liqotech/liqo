/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package foreignclusteroperator

import (
	"context"
	goerrors "errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/internal/discovery"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/clusterid"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	discoveryPkg "github.com/liqotech/liqo/pkg/discovery"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	peeringRoles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

const (
	noResourceRequestReason  = "NoResourceRequest"
	noResourceRequestMessage = "No ResourceRequest found in the Tenant Namespace %v"

	resourceRequestDeletingReason  = "ResourceRequestDeleting"
	resourceRequestDeletingMessage = "Ths ResourceRequest is in deleting phase in the Tenant Namespace %v"

	resourceRequestAcceptedReason  = "ResourceRequestAccepted"
	resourceRequestAcceptedMessage = "Ths ResourceRequest has been accepted by the remote cluster in the Tenant Namespace %v"

	resourceRequestCreatedReason  = "ResourceRequestCreated"
	resourceRequestCreatedMessage = "The ResourceRequest has been created in the Tenant Namespace %v"

	networkConfigNotFoundReason  = "NetworkConfigNotFound"
	networkConfigNotFoundMessage = "The NetworkConfig has not been found in the Tenant Namespace %v"

	networkConfigAvailableReason  = "NetworkConfigAvailable"
	networkConfigAvailableMessage = "The NetworkConfig has been successfully found and processed in the Tenant Namespace %v"

	networkConfigPendingReason  = "NetworkConfigPending"
	networkConfigPendingMessage = "The NetworkConfig has been found in the Tenant Namespace %v, but the processing has not completed yet"

	tunnelEndpointNotFoundReason  = "TunnelEndpointNotFound"
	tunnelEndpointNotFoundMessage = "The TunnelEndpointNotFound has not been found in the Tenant Namespace %v"

	tunnelEndpointAvailableReason  = "TunnelEndpointAvailable"
	tunnelEndpointAvailableMessage = "The TunnelEndpoint has been successfully found in the Tenant Namespace %v and it is connected"

	tunnelEndpointConnectingReason  = "TunnelEndpointConnecting"
	tunnelEndpointConnectingMessage = "The TunnelEndpoint has been successfully found in the Tenant Namespace %v, but it is not connected yet"

	tunnelEndpointErrorReason = "TunnelEndpointError"
)

// ForeignClusterReconciler reconciles a ForeignCluster object.
type ForeignClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	LiqoNamespacedClient client.Client
	clusterID            clusterid.ClusterID
	RequeueAfter         time.Duration

	namespaceManager tenantnamespace.Manager
	identityManager  identitymanager.IdentityManager

	peeringPermission peeringRoles.PeeringPermission

	ConfigProvider     discovery.ConfigProvider
	AuthConfigProvider auth.ConfigProvider
}

// clusterRole
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=searchdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=peeringrequests,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs,verbs=*
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs/status,verbs=*
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;watch;update
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=advertisements,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=advertisements/status,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=list
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;create
// tenant namespace management
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;delete;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;create;deletecollection;delete
// role
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=configmaps,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=serviceaccounts,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=secrets,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="liqo",resources=roles,verbs=get;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="liqo",resources=rolebindings,verbs=get;create

// Reconcile reconciles ForeignCluster resources.
func (r *ForeignClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	klog.V(4).Infof("Reconciling ForeignCluster %s", req.Name)

	tracer := trace.New("Reconcile", trace.Field{Key: "ForeignCluster", Value: req.Name})
	ctx = trace.ContextWithTrace(ctx, tracer)
	defer tracer.LogIfLong(traceutils.LongThreshold())

	var foreignCluster discoveryv1alpha1.ForeignCluster
	if err := r.Client.Get(ctx, req.NamespacedName, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	tracer.Step("Retrieved the foreign cluster")

	// ------ (1) validation ------

	// set labels and validate the resource spec
	if cont, res, err := r.validateForeignCluster(ctx, &foreignCluster); !cont {
		tracer.Step("Validated foreign cluster", trace.Field{Key: "requeuing", Value: true})
		return res, err
	}
	tracer.Step("Validated foreign cluster", trace.Field{Key: "requeuing", Value: false})

	// defer the status update function
	defer func() {
		defer tracer.Step("ForeignCluster status update")
		if newErr := r.Client.Status().Update(ctx, &foreignCluster); newErr != nil {
			klog.Error(newErr)
			err = newErr
		}
	}()

	// ensure that there are not multiple clusters with the same clusterID
	if processable, err := r.isClusterProcessable(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	} else if !processable {
		klog.Warningf("[%v] ClusterID not processable (%v): %v",
			foreignCluster.Spec.ClusterIdentity.ClusterID,
			foreignCluster.Name,
			peeringconditionsutils.GetMessage(&foreignCluster, discoveryv1alpha1.ProcessForeignClusterStatusCondition))
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, nil
	}
	tracer.Step("Ensured the ForeignCluster is processable")

	// ------ (2) ensuring prerequirements ------

	// ensure the existence of the local TenantNamespace
	if err = r.ensureLocalTenantNamespace(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	tracer.Step("Ensured the existence of the local tenant namespace")

	// ensure the existence of an identity to operate in the remote cluster remote cluster
	if err = r.ensureRemoteIdentity(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	tracer.Step("Ensured the existence of the remote identity")

	// fetch the remote tenant namespace name
	if err = r.fetchRemoteTenantNamespace(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	tracer.Step("Fetched the remote tenant namespace name")

	// ------ (3) peering/unpeering logic ------

	// read the ForeignCluster status and ensure the peering state
	phase := r.getDesiredOutgoingPeeringState(ctx, &foreignCluster)
	tracer.Step("Fetched the desired peering state")
	switch phase {
	case desiredPeeringPhasePeering:
		if err = r.peerNamespaced(ctx, &foreignCluster); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
		tracer.Step("Peered with a remote cluster")
	case desiredPeeringPhaseUnpeering:
		if err = r.unpeerNamespaced(ctx, &foreignCluster); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
		tracer.Step("Unpeered from a remote cluster")
	default:
		err := fmt.Errorf("unknown phase %v", phase)
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// ------ (4) update peering conditions ------

	// check for NetworkConfigs
	if err = r.checkNetwork(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	tracer.Step("Checked the NetworkConfig status")

	// check for TunnelEndpoints
	if err = r.checkTEP(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	tracer.Step("Checked the TunnelEndpoint status")

	// check if peering request really exists on foreign cluster
	if err := r.checkPeeringStatus(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	tracer.Step("Checked the peering status")

	// ------ (5) ensuring permission ------

	// ensure the permission for the current peering phase
	if err = r.ensurePermission(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	tracer.Step("Ensured the necessary permissions are present")

	// ------ (6) garbage collection ------

	// check if this ForeignCluster needs to be deleted. It could happen, for example, if it has been discovered
	// thanks to incoming peeringRequest and it has no active connections
	if foreignclusterutils.HasToBeRemoved(&foreignCluster) {
		klog.Infof("[%v] Delete ForeignCluster %v with discovery type %v",
			foreignCluster.Spec.ClusterIdentity.ClusterID,
			foreignCluster.Name, foreignclusterutils.GetDiscoveryType(&foreignCluster))
		if err := r.Client.Delete(ctx, &foreignCluster); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
		klog.V(4).Infof("ForeignCluster %s successfully reconciled", foreignCluster.Name)
		return ctrl.Result{}, nil
	}
	tracer.Step("Performed ForeignCluster garbage collection")

	klog.V(4).Infof("ForeignCluster %s successfully reconciled", foreignCluster.Name)
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: r.RequeueAfter,
	}, nil
}

// peerNamespaced enables the peering creating the resources in the correct TenantNamespace.
func (r *ForeignClusterReconciler) peerNamespaced(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	// create ResourceRequest
	result, err := r.createResourceRequest(ctx, foreignCluster)
	if err != nil {
		klog.Error(err)
		return err
	}

	// if the resource request has been created
	if result == controllerutil.OperationResultCreated {
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.OutgoingPeeringCondition,
			discoveryv1alpha1.PeeringConditionStatusPending,
			resourceRequestCreatedReason, fmt.Sprintf(resourceRequestCreatedMessage, foreignCluster.Status.TenantNamespace.Local))
	}
	return nil
}

// unpeerNamespaced disables the peering deleting the resources in the correct TenantNamespace.
func (r *ForeignClusterReconciler) unpeerNamespaced(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	var resourceRequest discoveryv1alpha1.ResourceRequest
	err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: foreignCluster.Status.TenantNamespace.Local,
		Name:      r.clusterID.GetClusterID(),
	}, &resourceRequest)
	if errors.IsNotFound(err) {
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.OutgoingPeeringCondition,
			discoveryv1alpha1.PeeringConditionStatusNone,
			noResourceRequestReason, fmt.Sprintf(noResourceRequestMessage, foreignCluster.Status.TenantNamespace.Local))
		return nil
	}
	if err != nil {
		klog.Error(err)
		return err
	}

	if resourceRequest.Status.OfferWithdrawalTimestamp.IsZero() {
		if resourceRequest.Spec.WithdrawalTimestamp.IsZero() {
			now := metav1.Now()
			resourceRequest.Spec.WithdrawalTimestamp = &now
		}
		err = r.Client.Update(ctx, &resourceRequest)
		if err != nil && !errors.IsNotFound(err) {
			klog.Error(err)
			return err
		}
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.OutgoingPeeringCondition,
			discoveryv1alpha1.PeeringConditionStatusDisconnecting,
			resourceRequestDeletingReason, fmt.Sprintf(resourceRequestDeletingMessage, foreignCluster.Status.TenantNamespace.Local))
	} else {
		err = r.deleteResourceRequest(ctx, foreignCluster)
		if err != nil && !errors.IsNotFound(err) {
			klog.Error(err)
			return err
		}
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.OutgoingPeeringCondition,
			discoveryv1alpha1.PeeringConditionStatusNone,
			noResourceRequestReason, fmt.Sprintf(noResourceRequestMessage, foreignCluster.Status.TenantNamespace.Local))
	}
	return nil
}

// SetupWithManager assigns the operator to a manager.
func (r *ForeignClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Prevent triggering a reconciliation in case of status modifications only.
	foreignClusterPredicate := predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.ForeignCluster{}, builder.WithPredicates(foreignClusterPredicate)).
		Owns(&discoveryv1alpha1.ResourceRequest{}).
		Complete(r)
}

func (r *ForeignClusterReconciler) checkPeeringStatus(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	var outgoingResourceRequestList discoveryv1alpha1.ResourceRequestList
	remoteClusterID := foreignCluster.Spec.ClusterIdentity.ClusterID
	localNamespace := foreignCluster.Status.TenantNamespace.Local
	if err := r.Client.List(ctx, &outgoingResourceRequestList, client.MatchingLabels(resourceRequestLabels(remoteClusterID)),
		client.InNamespace(localNamespace)); err != nil {
		klog.Error(err)
		return err
	}

	var incomingResourceRequestList discoveryv1alpha1.ResourceRequestList
	if err := r.Client.List(ctx, &incomingResourceRequestList, client.HasLabels{
		crdreplicator.ReplicationStatuslabel}, client.MatchingLabels{
		crdreplicator.RemoteLabelSelector: remoteClusterID,
	}); err != nil {
		klog.Error(err)
		return err
	}

	status, reason, message, err := getPeeringPhase(foreignCluster, &outgoingResourceRequestList)
	if err != nil {
		err = fmt.Errorf("[%v] %w in namespace %v", remoteClusterID, err, localNamespace)
		klog.Error(err)
		return err
	}
	peeringconditionsutils.EnsureStatus(foreignCluster,
		discoveryv1alpha1.OutgoingPeeringCondition, status, reason, message)

	status, reason, message, err = getPeeringPhase(foreignCluster, &incomingResourceRequestList)
	if err != nil {
		err = fmt.Errorf("[%v] %w in namespace %v", remoteClusterID, err, localNamespace)
		klog.Error(err)
		return err
	}
	peeringconditionsutils.EnsureStatus(foreignCluster,
		discoveryv1alpha1.IncomingPeeringCondition, status, reason, message)
	return nil
}

func getPeeringPhase(foreignCluster *discoveryv1alpha1.ForeignCluster,
	resourceRequestList *discoveryv1alpha1.ResourceRequestList) (status discoveryv1alpha1.PeeringConditionStatusType,
	reason, message string, err error) {
	switch len(resourceRequestList.Items) {
	case 0:
		return discoveryv1alpha1.PeeringConditionStatusNone, noResourceRequestReason,
			fmt.Sprintf(noResourceRequestMessage, foreignCluster.Status.TenantNamespace.Local), nil
	case 1:
		resourceRequest := &resourceRequestList.Items[0]
		desiredDelete := !resourceRequest.Spec.WithdrawalTimestamp.IsZero()
		deleted := !resourceRequest.Status.OfferWithdrawalTimestamp.IsZero()
		if desiredDelete || deleted {
			return discoveryv1alpha1.PeeringConditionStatusDisconnecting, resourceRequestDeletingReason,
				fmt.Sprintf(resourceRequestDeletingMessage, foreignCluster.Status.TenantNamespace.Local), nil
		}
		return discoveryv1alpha1.PeeringConditionStatusEstablished, resourceRequestAcceptedReason,
			fmt.Sprintf(resourceRequestAcceptedMessage, foreignCluster.Status.TenantNamespace.Local), nil
	default:
		err = fmt.Errorf("more than one resource request found")
		return discoveryv1alpha1.PeeringConditionStatusNone, noResourceRequestReason,
			fmt.Sprintf(noResourceRequestMessage, foreignCluster.Status.TenantNamespace.Local), err
	}
}

// get the external address where the Authentication Service is reachable from the external world.
func (r *ForeignClusterReconciler) getAddress() (string, error) {
	// this address can be overwritten setting this environment variable
	address := r.ConfigProvider.GetConfig().AuthServiceAddress
	if address != "" {
		return address, nil
	}

	// get the authentication service  (the namespace is automatically inferred by the namespaced client).
	var svc corev1.Service
	ref := types.NamespacedName{Name: discovery.AuthServiceName}
	if err := r.LiqoNamespacedClient.Get(context.TODO(), ref, &svc); err != nil {
		klog.Error(err)
		return "", err
	}

	// if the service is exposed as LoadBalancer
	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		// get the IP from the LoadBalancer service
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			// the service has no external IPs
			err := goerrors.New("no valid external IP for LoadBalancer Service")
			klog.Error(err)
			return "", err
		}
		lbIngress := svc.Status.LoadBalancer.Ingress[0]
		// return the external service IP
		if hostname := lbIngress.Hostname; hostname != "" {
			return hostname, nil
		} else if ip := lbIngress.IP; ip != "" {
			return ip, nil
		} else {
			// the service has no external IPs
			err := goerrors.New("no valid external IP for LoadBalancer Service")
			klog.Error(err)
			return "", err
		}
	}

	// only physical nodes
	//
	// we need to get an address from a physical node, if we have established peerings in the past with other clusters,
	// we may have some virtual nodes in our cluster. Since their IPs will not be reachable from other clusters, we cannot use them
	// as address for a local NodePort Service
	req, err := labels.NewRequirement(liqoconst.TypeLabel, selection.NotIn, []string{liqoconst.TypeNode})
	utilruntime.Must(err)

	// get the IP from the Nodes, to be used with NodePort services
	nodes := corev1.NodeList{}
	if err := r.Client.List(context.TODO(), &nodes, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(*req)}); err != nil {
		klog.Error(err)
		return "", err
	}

	if len(nodes.Items) == 0 {
		// there are no node is the cluster, we cannot get the address on any of them
		err = errors.NewNotFound(corev1.Resource("nodes"), "")
		klog.Error(err)
		return "", err
	}

	node := nodes.Items[0]
	return discoveryPkg.GetAddress(&node)

	// when an error occurs, it means that we was not able to get an address in any of the previous cases:
	// 1. no overwrite variable is set
	// 2. the service is not of type LoadBalancer
	// 3. there are no nodes in the cluster to get the IP for a NodePort service
}

// get the external port where the Authentication Service is reachable from the external world.
func (r *ForeignClusterReconciler) getPort() (string, error) {
	// this port can be overwritten setting this environment variable
	port := r.ConfigProvider.GetConfig().AuthServicePort
	if port != "" {
		return port, nil
	}

	// get the authentication service (the namespace is automatically inferred by the namespaced client).
	var svc corev1.Service
	ref := types.NamespacedName{Name: discovery.AuthServiceName}
	if err := r.LiqoNamespacedClient.Get(context.TODO(), ref, &svc); err != nil {
		klog.Error(err)
		return "", err
	}

	if len(svc.Spec.Ports) == 0 {
		// the service has no available port, we cannot get it
		err := errors.NewNotFound(corev1.Resource(string(corev1.ResourceServices)), discovery.AuthServiceName)
		klog.Error(err)
		return "", err
	}

	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		// return the LoadBalancer service external port
		return fmt.Sprintf("%v", svc.Spec.Ports[0].Port), nil
	}
	if svc.Spec.Type == corev1.ServiceTypeNodePort {
		// return the NodePort service port
		return fmt.Sprintf("%v", svc.Spec.Ports[0].NodePort), nil
	}
	// other service types. When we are using an Ingress we should not reach this code, because of the environment variable
	return "",
		fmt.Errorf(
			"you cannot expose the Auth Service with a %v Service. If you are using an Ingress, probably, there are configuration issues",
			svc.Spec.Type)
}

func (r *ForeignClusterReconciler) getHomeAuthURL() (string, error) {
	address, err := r.getAddress()
	if err != nil {
		return "", err
	}

	port, err := r.getPort()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://%s:%v", address, port), nil
}

func (r *ForeignClusterReconciler) checkNetwork(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	// local NetworkConfig
	labelSelector := map[string]string{crdreplicator.DestinationLabel: foreignCluster.Spec.ClusterIdentity.ClusterID}
	if err := r.updateNetwork(ctx,
		labelSelector, foreignCluster, discoveryv1alpha1.NetworkStatusCondition); err != nil {
		klog.Error(err)
		return err
	}

	// remote NetworkConfig
	labelSelector = map[string]string{crdreplicator.RemoteLabelSelector: foreignCluster.Spec.ClusterIdentity.ClusterID}
	if err := r.updateNetwork(ctx, labelSelector, foreignCluster, discoveryv1alpha1.NetworkStatusCondition); err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

func (r *ForeignClusterReconciler) updateNetwork(ctx context.Context,
	labelSelector map[string]string, foreignCluster *discoveryv1alpha1.ForeignCluster,
	conditionType discoveryv1alpha1.PeeringConditionType) error {
	var netList netv1alpha1.NetworkConfigList
	if err := r.Client.List(ctx, &netList, client.MatchingLabels(labelSelector)); err != nil {
		klog.Error(err)
		return err
	}
	if len(netList.Items) == 0 {
		// no NetworkConfigs found
		peeringconditionsutils.EnsureStatus(foreignCluster,
			conditionType, discoveryv1alpha1.PeeringConditionStatusNone,
			networkConfigNotFoundReason, fmt.Sprintf(networkConfigNotFoundMessage, foreignCluster.Status.TenantNamespace.Local))
	} else if len(netList.Items) > 0 && !isTunnelEndpointReason(peeringconditionsutils.GetReason(foreignCluster, conditionType)) {
		// there are NetworkConfigs
		ncf := &netList.Items[0]
		if ncf.Status.Processed {
			peeringconditionsutils.EnsureStatus(foreignCluster,
				conditionType, discoveryv1alpha1.PeeringConditionStatusEstablished,
				networkConfigAvailableReason, fmt.Sprintf(networkConfigAvailableMessage, foreignCluster.Status.TenantNamespace.Local))
		} else {
			peeringconditionsutils.EnsureStatus(foreignCluster,
				conditionType, discoveryv1alpha1.PeeringConditionStatusPending,
				networkConfigPendingReason, fmt.Sprintf(networkConfigPendingMessage, foreignCluster.Status.TenantNamespace.Local))
		}
	}
	return nil
}

func (r *ForeignClusterReconciler) checkTEP(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	var tepList netv1alpha1.TunnelEndpointList
	if err := r.Client.List(ctx, &tepList, client.MatchingLabels{
		liqoconst.ClusterIDLabelName: foreignCluster.Spec.ClusterIdentity.ClusterID,
	}); err != nil {
		klog.Error(err)
		return err
	}

	if len(tepList.Items) == 0 && peeringconditionsutils.GetReason(foreignCluster,
		discoveryv1alpha1.NetworkStatusCondition) == networkConfigAvailableReason {
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.NetworkStatusCondition, discoveryv1alpha1.PeeringConditionStatusNone,
			tunnelEndpointNotFoundReason, fmt.Sprintf(tunnelEndpointNotFoundMessage, foreignCluster.Status.TenantNamespace.Local))
	} else if len(tepList.Items) > 0 {
		tep := &tepList.Items[0]
		switch tep.Status.Connection.Status {
		case netv1alpha1.Connected:
			peeringconditionsutils.EnsureStatus(foreignCluster,
				discoveryv1alpha1.NetworkStatusCondition, discoveryv1alpha1.PeeringConditionStatusEstablished,
				tunnelEndpointAvailableReason, fmt.Sprintf(tunnelEndpointAvailableMessage, foreignCluster.Status.TenantNamespace.Local))
		case netv1alpha1.Connecting:
			peeringconditionsutils.EnsureStatus(foreignCluster,
				discoveryv1alpha1.NetworkStatusCondition, discoveryv1alpha1.PeeringConditionStatusPending,
				tunnelEndpointConnectingReason, fmt.Sprintf(tunnelEndpointConnectingMessage, foreignCluster.Status.TenantNamespace.Local))
		case netv1alpha1.ConnectionError:
			peeringconditionsutils.EnsureStatus(foreignCluster,
				discoveryv1alpha1.NetworkStatusCondition, discoveryv1alpha1.PeeringConditionStatusNone,
				tunnelEndpointErrorReason, tep.Status.Connection.StatusMessage)
		}
	}
	return nil
}

func isTunnelEndpointReason(reason string) bool {
	switch reason {
	case tunnelEndpointNotFoundReason, tunnelEndpointAvailableReason, tunnelEndpointConnectingReason, tunnelEndpointErrorReason:
		return true
	default:
		return false
	}
}
