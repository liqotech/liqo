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

package foreignclustercontroller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

// ForeignClusterReconciler reconciles a ForeignCluster object.
type ForeignClusterReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	ResyncPeriod time.Duration

	NetworkingEnabled     bool
	AuthenticationEnabled bool
	OffloadingEnabled     bool

	// Handle concurrent access to the map containing the cancel context functions of the API server checkers.
	APIServerCheckers
}

// clusterRole
// +kubebuilder:rbac:groups=core.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.liqo.io,resources=foreignclusters/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.liqo.io,resources=foreignclusters/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=connections,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayclients,verbs=get;list;watch
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=tenants,verbs=get;list;watch
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=identities,verbs=get;list;watch
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=virtualnodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;

// Reconcile reconciles ForeignCluster resources.
func (r *ForeignClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	klog.V(4).Infof("Reconciling ForeignCluster %s", req.Name)

	tracer := trace.New("Reconcile", trace.Field{Key: "ForeignCluster", Value: req.Name})
	ctx = trace.ContextWithTrace(ctx, tracer)
	defer tracer.LogIfLong(traceutils.LongThreshold())

	var foreignCluster liqov1beta1.ForeignCluster
	if err := r.Get(ctx, req.NamespacedName, &foreignCluster); err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("foreignCluster %q not found", req.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf("unable to get foreignCluster %q: %v", req.Name, err)
		return ctrl.Result{}, err
	}
	tracer.Step("Retrieved the foreign cluster")

	clusterID := foreignCluster.Spec.ClusterID
	if clusterID == "" {
		err := fmt.Errorf("foreignCluster %q has no clusterID", req.Name)
		klog.Error(err)
		return ctrl.Result{}, err
	}

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

	// Ensure that there are not multiple clusters with the same clusterID.
	// TODO: check on every top-level resources that there are no duplicate resources with the same
	// clusterID. To implement in webhooks.

	// Starts without making assumptions about the status of the ForeignCluster.
	// Keep only the conditions to preserve the last transition time.
	consumer, provider := false, false
	r.clearStatusExceptConditions(&foreignCluster)

	// Set conditions for each module.
	// The role of the foreigncluster is inferred from the retrieved resources.
	if err := r.handleNetworkingModuleStatus(ctx, &foreignCluster, r.NetworkingEnabled); err != nil {
		return ctrl.Result{}, err
	}
	tracer.Step("Handled networking module resources")

	if err := r.handleAuthenticationModuleStatus(ctx, &foreignCluster, r.AuthenticationEnabled, &consumer, &provider); err != nil {
		return ctrl.Result{}, err
	}
	tracer.Step("Handled authentication module resources")

	if err := r.handleOffloadingModuleStatus(ctx, &foreignCluster, r.OffloadingEnabled, &provider); err != nil {
		return ctrl.Result{}, err
	}
	tracer.Step("Handled offloading module resources")

	// Set the role of the ForeignCluster depending on the presence of the different resources.
	fcutils.SetRole(&foreignCluster, consumer, provider)

	// Activate/deactivate API server checker logic if the foreigncluster has the API server URL (or the proxy) set.
	cont, res, err := r.handleAPIServerChecker(ctx, &foreignCluster)
	if !cont || err != nil {
		// Prevent the deferred updateStatus() function to do an update since the FC has already been updated
		// or an error have occurred
		updateNeeded = false
		return res, err
	}

	klog.V(4).Infof("ForeignCluster %s successfully reconciled", foreignCluster.Name)

	return ctrl.Result{Requeue: true, RequeueAfter: r.ResyncPeriod}, nil
}

// SetupWithManager assigns the operator to a manager.
func (r *ForeignClusterReconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	// Prevent triggering a reconciliation in case of status modifications only.
	foreignClusterPredicate := predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})

	// Filter only Nodes created from VirtualNodes.
	virtualNodePredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: map[string]string{
		consts.TypeLabel: consts.TypeNode,
	}})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlForeignCluster).
		For(&liqov1beta1.ForeignCluster{}, builder.WithPredicates(foreignClusterPredicate)).
		Watches(&networkingv1beta1.Configuration{}, handler.EnqueueRequestsFromMapFunc(r.foreignclusterEnqueuer)).
		Watches(&networkingv1beta1.Connection{}, handler.EnqueueRequestsFromMapFunc(r.foreignclusterEnqueuer)).
		Watches(&networkingv1beta1.GatewayServer{}, handler.EnqueueRequestsFromMapFunc(r.foreignclusterEnqueuer)).
		Watches(&networkingv1beta1.GatewayClient{}, handler.EnqueueRequestsFromMapFunc(r.foreignclusterEnqueuer)).
		Watches(&authv1beta1.Tenant{}, handler.EnqueueRequestsFromMapFunc(r.foreignclusterEnqueuer)).
		Watches(&authv1beta1.Identity{}, handler.EnqueueRequestsFromMapFunc(r.foreignclusterEnqueuer)).
		Watches(&offloadingv1beta1.VirtualNode{}, handler.EnqueueRequestsFromMapFunc(r.foreignclusterEnqueuer)).
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.foreignclusterEnqueuer), builder.WithPredicates(virtualNodePredicate)).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}

func (r *ForeignClusterReconciler) foreignclusterEnqueuer(ctx context.Context, obj client.Object) []ctrl.Request {
	gvks, _, err := r.Scheme.ObjectKinds(obj)
	// Should never happen, but if it happens we panic.
	utilruntime.Must(err)

	// If gvk is found we log.
	if len(gvks) != 0 {
		klog.V(4).Infof("handling resource %q of type %q", klog.KObj(obj), gvks[0].String())
	}

	clusterID, ok := utils.GetClusterIDFromLabels(obj.GetLabels())
	if !ok {
		klog.V(4).Infof("resource %q has no clusterID label", klog.KObj(obj))
		return nil
	}

	// Get the foreigncluster. If it does not exist, create it, otherwise enqueue it.
	fc, err := fcutils.GetForeignClusterByID(ctx, r.Client, clusterID)
	switch {
	case errors.IsNotFound(err):
		// Create ForeignCluster
		klog.V(4).Infof("creating foreigncluster %q", clusterID)
		fc = &liqov1beta1.ForeignCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: string(clusterID),
				Labels: map[string]string{
					consts.RemoteClusterID: string(clusterID),
				},
			},
			Spec: liqov1beta1.ForeignClusterSpec{
				ClusterID: clusterID,
			},
		}
		if err := r.Create(ctx, fc); err != nil {
			klog.Errorf("an error occurred while creating foreigncluster %q: %s", clusterID, err)
			return nil
		}
		klog.Infof("Created foreigncluster %q", clusterID)
		return nil
	case err != nil:
		klog.Errorf("an error occurred while getting foreigncluster %q: %s", clusterID, err)
		return nil
	default:
		klog.V(4).Infof("enqueuing foreigncluster %q", clusterID)
		return []ctrl.Request{{NamespacedName: types.NamespacedName{Name: fc.Name}}}
	}
}
