// Copyright 2019-2022 The Liqo Authors
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

package foreignclusteroperator

import (
	"context"
	"fmt"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	resourcerequestoperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller"
	peeringRoles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/authenticationtoken"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

const (
	noResourceRequestReason  = "NoResourceRequest"
	noResourceRequestMessage = "No ResourceRequest found in the Tenant Namespace %v"

	resourceRequestDeletingReason  = "ResourceRequestDeleting"
	resourceRequestDeletingMessage = "The ResourceRequest is in deleting phase in the Tenant Namespace %v"

	resourceRequestAcceptedReason  = "ResourceRequestAccepted"
	resourceRequestAcceptedMessage = "The ResourceRequest has been accepted by the remote cluster in the Tenant Namespace %v"

	resourceRequestPendingReason  = "ResourceRequestPending"
	resourceRequestPendingMessage = "The remote cluster has not created a ResourceOffer in the Tenant Namespace %v yet"

	virtualKubeletPendingReason  = "KubeletPending"
	virtualKubeletPendingMessage = "The remote cluster has not started the VirtualKubelet for the peering yet"

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

	LiqoNamespace string

	ResyncPeriod               time.Duration
	HomeCluster                discoveryv1alpha1.ClusterIdentity
	AuthServiceAddressOverride string
	AuthServicePortOverride    string
	AutoJoin                   bool

	NamespaceManager tenantnamespace.Manager
	IdentityManager  identitymanager.IdentityManager

	PeeringPermission peeringRoles.PeeringPermission

	InsecureTransport *http.Transport
	SecureTransport   *http.Transport
}

// clusterRole
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=searchdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=searchdomains/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests/status,verbs=create;delete;deletecollection;list;watch
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers/status,verbs=create;delete;deletecollection;list;watch
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs,verbs=*
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs/status,verbs=*
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;watch;update
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;create
// tenant namespace management
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;delete;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;create;deletecollection;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;create;delete
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

	updateStatus := func() {
		defer tracer.Step("ForeignCluster status update")
		if newErr := r.Client.Status().Update(ctx, &foreignCluster); newErr != nil {
			klog.Error(newErr)
			err = newErr
		}
	}

	// ------ (1) validation ------

	// set labels and validate the resource spec
	cont, res, err := r.validateForeignCluster(ctx, &foreignCluster)
	if !cont || err != nil {
		tracer.Step("Validated foreign cluster", trace.Field{Key: "requeuing", Value: true})
		if err != nil {
			klog.Error(err)
			r.setForeignClusterStatusOnAuthUnavailable(&foreignCluster)
			updateStatus()
		}
		return res, err
	}
	tracer.Step("Validated foreign cluster", trace.Field{Key: "requeuing", Value: false})

	// defer the status update function
	defer updateStatus()

	// ensure that there are not multiple clusters with the same clusterID.
	// ensure that the foreign cluster proxy URL is a valid one is set.
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
			RequeueAfter: r.ResyncPeriod,
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
	if err = r.checkIncomingPeeringStatus(ctx, &foreignCluster); err != nil {
		klog.Error("[%s] %s", foreignCluster.Spec.ClusterIdentity.ClusterID, err)
		return ctrl.Result{}, err
	}
	tracer.Step("Checked the incoming peering status")

	// ------ (5) ensuring permission ------

	// ensure the permission for the current peering phase
	if err = r.ensurePermission(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	tracer.Step("Ensured the necessary permissions are present")

	// ------ (6) garbage collection ------

	// check if this ForeignCluster needs to be deleted. It could happen, for example, if it has been discovered
	// thanks to incoming resourceRequest and it has no active connections
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
		RequeueAfter: r.ResyncPeriod,
	}, nil
}

func (r *ForeignClusterReconciler) setForeignClusterStatusOnAuthUnavailable(foreignCluster *discoveryv1alpha1.ForeignCluster) {
	peeringconditionsutils.EnsureStatus(foreignCluster,
		discoveryv1alpha1.AuthenticationStatusCondition,
		discoveryv1alpha1.PeeringConditionStatusError,
		"AuthNotReachable",
		"The remote authentication service is not reachable")
}

// peerNamespaced enables the peering creating the resources in the correct TenantNamespace.
func (r *ForeignClusterReconciler) peerNamespaced(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	// Ensure the ResourceRequest is present
	resourceRequest, err := r.ensureResourceRequest(ctx, foreignCluster)
	if err != nil {
		klog.Error(err)
		return err
	}

	resourceOffer, err := r.getOutgoingResourceOffer(ctx, foreignCluster)
	if err != nil {
		return fmt.Errorf("reading resource offers: %w", err)
	}

	// Update the peering status based on the ResourceRequest status
	status, reason, message, err := getPeeringPhase(foreignCluster, resourceRequest, resourceOffer)
	if err != nil {
		err = fmt.Errorf("[%v] %w", foreignCluster.Spec.ClusterIdentity.ClusterID, err)
		klog.Error(err)
		return err
	}

	peeringconditionsutils.EnsureStatus(foreignCluster,
		discoveryv1alpha1.OutgoingPeeringCondition, status, reason, message)

	return nil
}

// unpeerNamespaced disables the peering deleting the resources in the correct TenantNamespace.
func (r *ForeignClusterReconciler) unpeerNamespaced(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	var resourceRequest discoveryv1alpha1.ResourceRequest
	err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: foreignCluster.Status.TenantNamespace.Local,
		Name:      getResourceRequestNameFor(r.HomeCluster),
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
func (r *ForeignClusterReconciler) SetupWithManager(mgr ctrl.Manager, workers uint) error {
	// Prevent triggering a reconciliation in case of status modifications only.
	foreignClusterPredicate := predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})

	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.ForeignCluster{}, builder.WithPredicates(foreignClusterPredicate)).
		Owns(&discoveryv1alpha1.ResourceRequest{}).
		Owns(&netv1alpha1.NetworkConfig{}).
		Owns(&netv1alpha1.TunnelEndpoint{}).
		Watches(&source.Kind{Type: &corev1.Secret{}}, authenticationtoken.GetAuthTokenSecretEventHandler(r.Client),
			builder.WithPredicates(authenticationtoken.GetAuthTokenSecretPredicate())).
		Watches(&source.Kind{Type: &sharingv1alpha1.ResourceOffer{}},
			handler.EnqueueRequestsFromMapFunc(r.resourceOfferHandler)).
		WithOptions(controller.Options{MaxConcurrentReconciles: int(workers)}).
		Complete(r)
}

func (r *ForeignClusterReconciler) resourceOfferHandler(obj client.Object) []ctrl.Request {
	ro := obj.(*sharingv1alpha1.ResourceOffer)
	var clusterID string
	if _, ok := ro.Labels[liqoconst.ReplicationStatusLabel]; ok { // incoming resource offer
		clusterID = ro.Labels[liqoconst.ReplicationOriginLabel]
	} else {
		clusterID = ro.Labels[liqoconst.ReplicationDestinationLabel]
	}
	remoteCluster, err := foreignclusterutils.GetForeignClusterByID(context.Background(), r.Client, clusterID)
	if err != nil {
		klog.Warningf("Could not handle resource offer %q update: %s", klog.KObj(ro), err)
		return []ctrl.Request{}
	}
	return []ctrl.Request{{NamespacedName: types.NamespacedName{Name: remoteCluster.Name}}}
}

func (r *ForeignClusterReconciler) checkIncomingPeeringStatus(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	remoteClusterID := foreignCluster.Spec.ClusterIdentity.ClusterID
	localNamespace := foreignCluster.Status.TenantNamespace.Local

	incomingResourceRequest, err := resourcerequestoperator.GetResourceRequest(ctx, r.Client, remoteClusterID)
	if err != nil {
		return fmt.Errorf("reading resource requests: %w", err)
	}

	resourceOffer, err := r.getIncomingResourceOffer(ctx, foreignCluster)
	if err != nil {
		return fmt.Errorf("reading resource offers: %w", err)
	}

	status, reason, message, err := getPeeringPhase(foreignCluster, incomingResourceRequest, resourceOffer)
	if err != nil {
		return fmt.Errorf("reading peering phase from namespace %s: %w", localNamespace, err)
	}
	peeringconditionsutils.EnsureStatus(foreignCluster,
		discoveryv1alpha1.IncomingPeeringCondition, status, reason, message)
	return nil
}

// getIncomingResourceOffer returns the ResourceOffer created for the given remote cluster in an incoming peering, or a
// nil pointer if there is none.
func (r *ForeignClusterReconciler) getIncomingResourceOffer(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) (*sharingv1alpha1.ResourceOffer, error) {
	return r.getResourceOfferWithLabels(ctx, []client.ListOption{client.MatchingLabels{
		liqoconst.ReplicationRequestedLabel:   "true",
		liqoconst.ReplicationDestinationLabel: foreignCluster.Spec.ClusterIdentity.ClusterID,
	}})
}

// getOutgoingResourceOffer returns the ResourceOffer created by the given remote cluster in an outgoing peering, or a
// nil pointer if there is none.
func (r *ForeignClusterReconciler) getOutgoingResourceOffer(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) (*sharingv1alpha1.ResourceOffer, error) {
	return r.getResourceOfferWithLabels(ctx, []client.ListOption{client.HasLabels{
		liqoconst.ReplicationStatusLabel}, client.MatchingLabels{
		liqoconst.ReplicationOriginLabel: foreignCluster.Spec.ClusterIdentity.ClusterID,
	}})
}

// getResourceOfferWithLabels returns the ResourceOffer with the given labels.
func (r *ForeignClusterReconciler) getResourceOfferWithLabels(ctx context.Context,
	labels []client.ListOption) (*sharingv1alpha1.ResourceOffer, error) {
	var resourceOfferList sharingv1alpha1.ResourceOfferList
	if err := r.Client.List(ctx, &resourceOfferList, labels...); err != nil {
		return nil, err
	}

	switch len(resourceOfferList.Items) {
	case 0:
		return nil, nil
	case 1:
		return &resourceOfferList.Items[0], nil
	default:
		return nil, fmt.Errorf("more than one resource offer found")
	}
}

func getPeeringPhase(foreignCluster *discoveryv1alpha1.ForeignCluster,
	resourceRequest *discoveryv1alpha1.ResourceRequest,
	resourceOffer *sharingv1alpha1.ResourceOffer) (status discoveryv1alpha1.PeeringConditionStatusType,
	reason, message string, err error) {
	if resourceRequest == nil {
		return discoveryv1alpha1.PeeringConditionStatusNone, noResourceRequestReason,
			fmt.Sprintf(noResourceRequestMessage, foreignCluster.Status.TenantNamespace.Local), nil
	}

	desiredDelete := !resourceRequest.Spec.WithdrawalTimestamp.IsZero()
	deleted := !resourceRequest.Status.OfferWithdrawalTimestamp.IsZero()
	offerState := resourceRequest.Status.OfferState

	// the offerState indicates if the ResourceRequest has been accepted and
	// the ResourceOffer has been created by the remote cluster.
	// * "Created" state means that the resources has been offered, and, if the withdrawal timestamp is set,
	//   the offered is created, but it is no more valid. -> the cluster is disconnecting.
	// * "None" there is no ResourceOffer created by the remote cluster, both because it didn't yet or because
	//   it did not accept the incoming peering from the local cluster. -> the peering state is Pending.
	switch offerState {
	case discoveryv1alpha1.OfferStateCreated:
		if desiredDelete || deleted {
			return discoveryv1alpha1.PeeringConditionStatusDisconnecting, resourceRequestDeletingReason,
				fmt.Sprintf(resourceRequestDeletingMessage, foreignCluster.Status.TenantNamespace.Local), nil
		}
		// Return "pending" if we sent the ResourceRequest but the foreign cluster did not yet create the ResourceOffer
		if resourceOffer == nil {
			return discoveryv1alpha1.PeeringConditionStatusPending, resourceRequestPendingReason,
				fmt.Sprintf(resourceRequestPendingMessage, foreignCluster.Status.TenantNamespace.Local), nil
		}
		if resourceOffer.Status.VirtualKubeletStatus != sharingv1alpha1.VirtualKubeletStatusCreated {
			return discoveryv1alpha1.PeeringConditionStatusPending, virtualKubeletPendingReason,
				virtualKubeletPendingMessage, nil
		}
		return discoveryv1alpha1.PeeringConditionStatusEstablished, resourceRequestAcceptedReason,
			fmt.Sprintf(resourceRequestAcceptedMessage, foreignCluster.Status.TenantNamespace.Local), nil
	case discoveryv1alpha1.OfferStateNone, "":
		return discoveryv1alpha1.PeeringConditionStatusPending, resourceRequestPendingReason,
			fmt.Sprintf(resourceRequestPendingMessage, foreignCluster.Status.TenantNamespace.Local), nil
	default:
		err = fmt.Errorf("unknown offer state %v", offerState)
		return discoveryv1alpha1.PeeringConditionStatusNone, noResourceRequestReason,
			fmt.Sprintf(noResourceRequestMessage, foreignCluster.Status.TenantNamespace.Local), err
	}
}

func (r *ForeignClusterReconciler) checkNetwork(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	// local NetworkConfig
	labelSelector := map[string]string{liqoconst.ReplicationDestinationLabel: foreignCluster.Spec.ClusterIdentity.ClusterID}
	if established, err := r.updateNetwork(ctx,
		labelSelector, foreignCluster, discoveryv1alpha1.NetworkStatusCondition); err != nil {
		klog.Error(err)
		return err
	} else if !established {
		// Given the first network config is not ready, it is not necessary to check the second
		return nil
	}

	// remote NetworkConfig
	labelSelector = map[string]string{liqoconst.ReplicationOriginLabel: foreignCluster.Spec.ClusterIdentity.ClusterID}
	if _, err := r.updateNetwork(ctx, labelSelector, foreignCluster, discoveryv1alpha1.NetworkStatusCondition); err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

func (r *ForeignClusterReconciler) updateNetwork(ctx context.Context,
	labelSelector map[string]string, foreignCluster *discoveryv1alpha1.ForeignCluster,
	conditionType discoveryv1alpha1.PeeringConditionType) (established bool, err error) {
	var netList netv1alpha1.NetworkConfigList
	if err = r.Client.List(ctx, &netList, client.MatchingLabels(labelSelector)); err != nil {
		klog.Error(err)
		return false, err
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
			established = true
			peeringconditionsutils.EnsureStatus(foreignCluster,
				conditionType, discoveryv1alpha1.PeeringConditionStatusEstablished,
				networkConfigAvailableReason, fmt.Sprintf(networkConfigAvailableMessage, foreignCluster.Status.TenantNamespace.Local))
		} else {
			peeringconditionsutils.EnsureStatus(foreignCluster,
				conditionType, discoveryv1alpha1.PeeringConditionStatusPending,
				networkConfigPendingReason, fmt.Sprintf(networkConfigPendingMessage, foreignCluster.Status.TenantNamespace.Local))
		}
	}
	return established, nil
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
