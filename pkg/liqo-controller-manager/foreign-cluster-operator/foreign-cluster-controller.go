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

package foreignclusteroperator

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	peeringRoles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	getters "github.com/liqotech/liqo/pkg/utils/getters"
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

	connectionNotFoundReason  = "ConnectionNotFound"
	connectionNotFoundMessage = "The connection has not been found for the remote cluster %v"

	connectionAvailableReason  = "ConnectionAvailable"
	connectionAvailableMessage = "The connection has been found for the remote cluster %v"

	connectionConnectingReason  = "ConnectionConnecting"
	connectionConnectingMessage = "The connection has been found for the remote cluster %v, but it is not connected yet"

	connectionErrorReason  = "ConnectionError"
	connectionErrorMessage = "The connection has been found for the remote cluster %v, but an error occurred"

	externalNetworkReason  = "ExternalNetwork"
	externalNetworkMessage = "The remote cluster network connection is not managed by Liqo"

	apiServerReadyReason  = "APIServerReady"
	apiServerReadyMessage = "The remote cluster API Server is ready"

	apiServerNotReadyReason  = "APIServerNotReady"
	apiServerNotReadyMessage = "The remote cluster API Server is not ready"
)

// ForeignClusterReconciler reconciles a ForeignCluster object.
type ForeignClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	LiqoNamespace string

	ResyncPeriod      time.Duration
	HomeCluster       discoveryv1alpha1.ClusterIdentity
	NetworkingEnabled bool

	NamespaceManager tenantnamespace.Manager
	IdentityManager  identitymanager.IdentityManager

	PeeringPermission peeringRoles.PeeringPermission

	InsecureTransport *http.Transport
	SecureTransport   *http.Transport

	// The map associates the local tenant namespaces (keys) to the related foreignclusters (values).
	ForeignClusters sync.Map

	// Handle concurrent access to the map containing the cancel context functions of the API server checkers.
	APIServerCheckers
}

// clusterRole
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests/status,verbs=create;delete;deletecollection;list;watch
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers/status,verbs=create;delete;deletecollection;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=connections,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=connections/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch
// tenant namespace management
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;delete;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;deletecollection;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// role
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="liqo",resources=roles,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="liqo",resources=rolebindings,verbs=get;list;watch;create;update;patch

// Reconcile reconciles ForeignCluster resources.
func (r *ForeignClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	klog.V(4).Infof("Reconciling ForeignCluster %s", req.Name)

	tracer := trace.New("Reconcile", trace.Field{Key: "ForeignCluster", Value: req.Name})
	ctx = trace.ContextWithTrace(ctx, tracer)
	defer tracer.LogIfLong(traceutils.LongThreshold())

	var foreignCluster discoveryv1alpha1.ForeignCluster
	if err := r.Client.Get(ctx, req.NamespacedName, &foreignCluster); err != nil && !errors.IsNotFound(err) {
		klog.Error(err)
		return ctrl.Result{}, err
	} else if errors.IsNotFound(err) {
		// If the foreigncluster has been removed than remove the mapping between the local tenant namespace and
		// the foreign cluster.
		r.ForeignClusters.Delete(foreignCluster.Status.TenantNamespace.Local)
		return ctrl.Result{}, nil
	}
	tracer.Step("Retrieved the foreign cluster")

	// This flag allows to disable the status update function in case it is deferred, but the foreign cluster
	// have already been updated, hence preventing an unnecessary and failing API call.
	updateNeeded := true
	updateStatus := func() {
		if updateNeeded {
			defer tracer.Step("ForeignCluster status update")
			if newErr := r.Client.Status().Update(ctx, &foreignCluster); newErr != nil {
				klog.Error(newErr)
				err = newErr
			}
		}
	}

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

	// check for Connection
	if err = r.checkConnection(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	tracer.Step("Checked the Connection status")

	// ------ (1) ensuring prerequirements ------

	// ensure the existence of the local TenantNamespace
	if err = r.ensureLocalTenantNamespace(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	tracer.Step("Ensured the existence of the local tenant namespace")

	// Add the foreigncluster to the map once the local tenant namespace has been created.
	r.ForeignClusters.Store(foreignCluster.Status.TenantNamespace.Local, foreignCluster.GetName())

	// ------ (2) activate/deactivate API server checker logic ------
	cont, res, err := r.handleAPIServerChecker(ctx, &foreignCluster)
	if !cont || err != nil {
		// Prevent the deferred updateStatus() function to do an update since the FC has already been updated
		// or an error have occurred
		updateNeeded = false

		tracer.Step("Handled the API server checker", trace.Field{Key: "requeuing", Value: true})
		return res, err
	}
	tracer.Step("Handled the API server checker", trace.Field{Key: "requeuing", Value: false})

	// ------ (3) update peering conditions ------
	// check if peering request really exists on foreign cluster
	if err = r.checkIncomingPeeringStatus(ctx, &foreignCluster); err != nil {
		klog.Errorf("[%s] %s", foreignCluster.Spec.ClusterIdentity.ClusterID, err)
		return ctrl.Result{}, err
	}
	tracer.Step("Checked the incoming peering status")

	// ------ (4) ensuring permission ------

	// ensure the permission for the current peering phase
	if err = r.ensurePermission(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}
	tracer.Step("Ensured the necessary permissions are present")

	klog.V(4).Infof("ForeignCluster %s successfully reconciled", foreignCluster.Name)
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: r.ResyncPeriod,
	}, nil
}

// SetupWithManager assigns the operator to a manager.
func (r *ForeignClusterReconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	// Prevent triggering a reconciliation in case of status modifications only.
	foreignClusterPredicate := predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})

	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.ForeignCluster{}, builder.WithPredicates(foreignClusterPredicate)).
		Watches(&networkingv1alpha1.Connection{}, handler.EnqueueRequestsFromMapFunc(r.clusterIDEnqueuer)).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}

func (r *ForeignClusterReconciler) checkIncomingPeeringStatus(_ context.Context,
	_ *discoveryv1alpha1.ForeignCluster) error {
	return nil
}

func ensureStatusForConnection(foreignCluster *discoveryv1alpha1.ForeignCluster, connection *networkingv1alpha1.Connection) {
	switch connection.Status.Value {
	case networkingv1alpha1.Connected:
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.NetworkStatusCondition, discoveryv1alpha1.PeeringConditionStatusEstablished,
			connectionAvailableReason, fmt.Sprintf(connectionAvailableMessage, foreignCluster.Spec.ClusterIdentity.ClusterID))
	case networkingv1alpha1.Connecting:
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.NetworkStatusCondition, discoveryv1alpha1.PeeringConditionStatusPending,
			connectionConnectingReason, fmt.Sprintf(connectionConnectingMessage, foreignCluster.Spec.ClusterIdentity.ClusterID))
	case networkingv1alpha1.ConnectionError:
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.NetworkStatusCondition, discoveryv1alpha1.PeeringConditionStatusError,
			connectionErrorReason, fmt.Sprintf(connectionErrorMessage, foreignCluster.Spec.ClusterIdentity.ClusterID))
	}
}

func (r *ForeignClusterReconciler) checkConnection(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	if !r.NetworkingEnabled {
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.NetworkStatusCondition, discoveryv1alpha1.PeeringConditionStatusDisabled,
			externalNetworkReason, externalNetworkMessage)
		return nil
	}

	remoteClusterIDSelector := labels.Set{consts.RemoteClusterID: foreignCluster.Spec.ClusterIdentity.ClusterID}.AsSelector()
	connections, err := getters.ListConnectionsByLabel(ctx, r.Client, corev1.NamespaceAll, remoteClusterIDSelector)
	if err != nil {
		klog.Error(err)
		return err
	}

	switch len(connections.Items) {
	case 0:
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.NetworkStatusCondition, discoveryv1alpha1.PeeringConditionStatusNone,
			connectionNotFoundReason, fmt.Sprintf(connectionNotFoundMessage, foreignCluster.Spec.ClusterIdentity.ClusterID))
	case 1:
		connection := &connections.Items[0]
		ensureStatusForConnection(foreignCluster, connection)
	default:
		// get the oldest connection
		sort.Slice(connections.Items, func(i, j int) bool {
			return connections.Items[i].CreationTimestamp.Before(&connections.Items[j].CreationTimestamp)
		})
		connection := &connections.Items[0]
		klog.Errorf("multiple connections found for remote cluster %v, using the oldest one %q",
			foreignCluster.Spec.ClusterIdentity.ClusterID, client.ObjectKeyFromObject(connection))
		ensureStatusForConnection(foreignCluster, connection)
	}

	return nil
}

func (r *ForeignClusterReconciler) clusterIDEnqueuer(ctx context.Context, obj client.Object) []ctrl.Request {
	if obj.GetLabels() == nil {
		return []ctrl.Request{}
	}

	clusterID, ok := obj.GetLabels()[consts.RemoteClusterID]
	if !ok {
		return []ctrl.Request{}
	}

	fc, err := foreignclusterutils.GetForeignClusterByID(ctx, r.Client, clusterID)
	if err != nil {
		klog.V(6).Info(err) // we only print the event, as the network can be created without the existence of the foreign cluster
		return []ctrl.Request{}
	}

	klog.V(4).Infof("enqueuing foreigncluster %q for object %q", fc.GetName(), klog.KObj(obj))
	return []ctrl.Request{{NamespacedName: types.NamespacedName{Name: fc.GetName()}}}
}

func (r *ForeignClusterReconciler) foreignclusterEnqueuer(_ context.Context, obj client.Object) []ctrl.Request {
	gvks, _, err := r.Scheme.ObjectKinds(obj)
	// Should never happen, but if it happens we panic.
	utilruntime.Must(err)

	// If gvk is found we log.
	if len(gvks) != 0 {
		klog.V(4).Infof("handling resource %q of type %q", klog.KObj(obj), gvks[0].String())
	}

	fcName, ok := r.ForeignClusters.Load(obj.GetNamespace())

	if !ok {
		klog.V(4).Infof("no foreigncluster found for resource %q", klog.KObj(obj))
		return []ctrl.Request{}
	}

	klog.V(4).Infof("enqueuing foreigncluster %q", fcName.(string))

	return []ctrl.Request{{NamespacedName: types.NamespacedName{Name: fcName.(string)}}}
}
