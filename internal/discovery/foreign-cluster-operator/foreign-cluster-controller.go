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

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/internal/discovery"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/clusterid"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	discoveryPkg "github.com/liqotech/liqo/pkg/discovery"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	peeringRoles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
)

// FinalizerString is added as finalizer for peered ForeignClusters.
const FinalizerString = "foreigncluster.discovery.liqo.io/peered"

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

	Namespace     string
	crdClient     *crdclient.CRDClient
	networkClient *crdclient.CRDClient
	clusterID     clusterid.ClusterID
	RequeueAfter  time.Duration

	namespaceManager tenantnamespace.Manager
	identityManager  identitymanager.IdentityManager

	peeringPermission peeringRoles.PeeringPermission

	ConfigProvider     discovery.ConfigProvider
	AuthConfigProvider auth.ConfigProvider

	// testing
	ForeignConfig *rest.Config
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
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=services,verbs=get
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=configmaps,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=serviceaccounts,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=secrets,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="liqo",resources=roles,verbs=get;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="liqo",resources=rolebindings,verbs=get;create

// Reconcile reconciles ForeignCluster resources.
func (r *ForeignClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	klog.V(4).Infof("Reconciling ForeignCluster %s", req.Name)

	var foreignCluster discoveryv1alpha1.ForeignCluster
	if err := r.Client.Get(ctx, req.NamespacedName, &foreignCluster); err != nil {
		klog.Error(err)
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// ------ (1) validation ------

	// set labels and validate the resource spec
	if cont, res, err := r.validateForeignCluster(ctx, &foreignCluster); !cont {
		return res, err
	}

	// defer the status update function
	defer func() {
		if newErr := r.Client.Status().Update(ctx, &foreignCluster); newErr != nil {
			klog.Error(newErr)
			err = newErr
		}
	}()

	// ------ (2) ensuring prerequirements ------

	// ensure the existence of the local TenantNamespace
	if err = r.ensureLocalTenantNamespace(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// ensure the existence of an identity to operate in the remote cluster remote cluster
	if err = r.ensureRemoteIdentity(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// fetch the remote tenant namespace name
	if err = r.fetchRemoteTenantNamespace(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// ------ (3) peering/unpeering logic ------

	// read the ForeignCluster status and ensure the peering state
	phase := r.getDesiredOutgoingPeeringState(ctx, &foreignCluster)
	switch phase {
	case desiredPeeringPhasePeering:
		if err = r.peerNamespaced(ctx, &foreignCluster); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
	case desiredPeeringPhaseUnpeering:
		if err = r.unpeerNamespaced(ctx, &foreignCluster); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
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

	// check for TunnelEndpoints
	if err = r.checkTEP(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// check if peering request really exists on foreign cluster
	if err := r.checkPeeringStatus(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// ------ (5) ensuring permission ------

	// ensure the permission for the current peering phase
	if err = r.ensurePermission(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// ------ (6) garbage collection ------

	// check if this ForeignCluster needs to be deleted. It could happen, for example, if it has been discovered
	// thanks to incoming peeringRequest and it has no active connections
	if foreignclusterutils.HasToBeRemoved(&foreignCluster) {
		klog.Infof("[%v] Delete ForeignCluster %v with discovery type %v",
			foreignCluster.Spec.ClusterIdentity.ClusterID,
			foreignCluster.Name, foreignclusterutils.GetDiscoveryType(&foreignCluster))
		if err := r.deleteForeignCluster(ctx, &foreignCluster); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
		klog.V(4).Infof("ForeignCluster %s successfully reconciled", foreignCluster.Name)
		return ctrl.Result{}, nil
	}

	klog.V(4).Infof("ForeignCluster %s successfully reconciled", foreignCluster.Name)
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: r.RequeueAfter,
	}, nil
}

func (r *ForeignClusterReconciler) update(fc *discoveryv1alpha1.ForeignCluster) (*discoveryv1alpha1.ForeignCluster, error) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if tmp, err := r.crdClient.Resource("foreignclusters").Update(fc.Name, fc, &metav1.UpdateOptions{}); err == nil {
			var ok bool
			fc, ok = tmp.(*discoveryv1alpha1.ForeignCluster)
			if !ok {
				err = goerrors.New("this object is not a ForeignCluster")
				klog.Error(err, tmp)
				return err
			}
			return nil
		} else if !errors.IsConflict(err) {
			return err
		}
		tmp, err := r.crdClient.Resource("foreignclusters").Get(fc.Name, &metav1.GetOptions{})
		if err != nil {
			klog.Error(err)
			return err
		}
		fc2, ok := tmp.(*discoveryv1alpha1.ForeignCluster)
		if !ok {
			err = goerrors.New("this object is not a ForeignCluster")
			klog.Error(err, tmp)
			return err
		}
		fc.ResourceVersion = fc2.ResourceVersion
		fc.Generation = fc2.Generation
		return err
	})
	return fc, err
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.ForeignCluster{}).
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

	// get the authentication service
	svc, err := r.crdClient.Client().CoreV1().Services(r.Namespace).Get(context.TODO(), discovery.AuthServiceName, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return "", err
	}
	// if the service is exposed as LoadBalancer
	if svc.Spec.Type == apiv1.ServiceTypeLoadBalancer {
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
	labelSelector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      liqoconst.TypeLabel,
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{liqoconst.TypeNode},
			},
		},
	})
	if err != nil {
		klog.Error(err)
		return "", err
	}

	// get the IP from the Nodes, to be used with NodePort services
	nodes, err := r.crdClient.Client().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		klog.Error(err)
		return "", err
	}
	if len(nodes.Items) == 0 {
		// there are no node is the cluster, we cannot get the address on any of them
		err = errors.NewNotFound(schema.GroupResource{
			Group:    apiv1.GroupName,
			Resource: "nodes",
		}, "")
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

	// get the authentication service
	svc, err := r.crdClient.Client().CoreV1().Services(r.Namespace).Get(context.TODO(), discovery.AuthServiceName, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return "", err
	}
	if len(svc.Spec.Ports) == 0 {
		// the service has no available port, we cannot get it
		err = errors.NewNotFound(schema.GroupResource{
			Group:    apiv1.GroupName,
			Resource: string(apiv1.ResourceServices),
		}, discovery.AuthServiceName)
		klog.Error(err)
		return "", err
	}

	if svc.Spec.Type == apiv1.ServiceTypeLoadBalancer {
		// return the LoadBalancer service external port
		return fmt.Sprintf("%v", svc.Spec.Ports[0].Port), nil
	}
	if svc.Spec.Type == apiv1.ServiceTypeNodePort {
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

func (r *ForeignClusterReconciler) deleteForeignCluster(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	return r.Client.Delete(ctx, foreignCluster)
}

func isTunnelEndpointReason(reason string) bool {
	switch reason {
	case tunnelEndpointNotFoundReason, tunnelEndpointAvailableReason, tunnelEndpointConnectingReason, tunnelEndpointErrorReason:
		return true
	default:
		return false
	}
}
