// Copyright 2019-2026 The Liqo Authors
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

package route

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vishvananda/netlink"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/network/netmonitor"
)

// RouteConfigurationBindingReconciler manages RouteConfigurationBinding lifecycle.
//
//nolint:revive // We usually use the name of the reconciled resource in the controller name.
type RouteConfigurationBindingReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder events.EventRecorder
}

// NewRouteConfigurationBindingReconciler returns a new RouteConfigurationBindingReconciler.
func NewRouteConfigurationBindingReconciler(cl client.Client, s *runtime.Scheme,
	er events.EventRecorder) *RouteConfigurationBindingReconciler {
	return &RouteConfigurationBindingReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurationbindings,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurationbindings/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurationbindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile manages RouteConfigurationBindings, applying netlink routes/rules/tables from the referenced RouteConfiguration.
func (r *RouteConfigurationBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	routebinding := &networkingv1beta1.RouteConfigurationBinding{}
	if err = r.Get(ctx, req.NamespacedName, routebinding); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(6).Infof("There is no routeconfigurationbinding %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting routeconfigurationbinding: %w", err)
	}

	klog.V(4).Infof("Reconciling routeconfigurationbinding %s", req.String())

	// Deletion path: use the table name cached in the status so the RouteConfiguration
	// does not need to be fetched (it may already be deleted).
	if !routebinding.DeletionTimestamp.IsZero() {
		if ctrlutil.ContainsFinalizer(routebinding, routeConfigurationBindingControllerFinalizer) {
			if routebinding.Status.TableName != "" {
				tableID, tableErr := GetTableID(routebinding.Status.TableName)
				if tableErr == nil {
					r.cleanupTable(tableID)
					klog.Infof("Deleted netlink configuration for routeconfigurationbinding %s", req.String())
				} else {
					klog.Warningf("Unable to get table ID for routeconfigurationbinding %s: %v", req.String(), tableErr)
				}
			}
			if err = r.ensureBindingFinalizerAbsence(ctx, routebinding); err != nil {
				return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
			}
			klog.Infof("Removed finalizer from routeconfigurationbinding %s", req.String())
		}
		return ctrl.Result{}, nil
	}

	if !ctrlutil.ContainsFinalizer(routebinding, routeConfigurationBindingControllerFinalizer) {
		if err = r.ensureBindingFinalizerPresence(ctx, routebinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// From here on the status will be updated to reflect the reconcile result.
	defer func() {
		updateErr := r.updateStatus(ctx, routebinding, err)
		if updateErr != nil {
			updateErr = fmt.Errorf("updating status: %w", updateErr)
			err = errors.Join(err, updateErr)
		}
	}()

	// Normal path: fetch the RouteConfiguration to get the full table spec.
	routecfg := &networkingv1beta1.RouteConfiguration{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      routebinding.Spec.RouteConfigurationRef.Name,
		Namespace: routebinding.Namespace,
	}, routecfg)

	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("Referenced routeconfiguration %q not found for binding %s; requeuing",
				routebinding.Spec.RouteConfigurationRef.Name, req.String())
			return ctrl.Result{}, fmt.Errorf("referenced routeconfiguration %q not found",
				routebinding.Spec.RouteConfigurationRef.Name)
		}
		return ctrl.Result{}, fmt.Errorf("getting referenced routeconfiguration: %w", err)
	}

	// Cache the table name in the status so it is available during cleanup even after
	// the RouteConfiguration has been deleted.
	routebinding.Status.TableName = routecfg.Spec.Table.Name

	tableID, err := GetTableID(routecfg.Spec.Table.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting the table ID: %w", err)
	}

	klog.V(4).Infof("Applying routeconfigurationbinding %s (routecfg %s)", req.String(), routecfg.Name)

	existingRules, err := GetRulesByTableID(tableID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("listing existing rules: %w", err)
	}
	if err = CleanRules(routecfg.Spec.Table.Rules, existingRules); err != nil {
		return ctrl.Result{}, fmt.Errorf("cleaning rules: %w", err)
	}

	existingRoutes, err := GetRoutesByTableID(tableID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("listing existing routes: %w", err)
	}

	allRoutes := []networkingv1beta1.Route{}
	for i := range routecfg.Spec.Table.Rules {
		allRoutes = append(allRoutes, routecfg.Spec.Table.Rules[i].Routes...)
	}
	if err = CleanRoutes(allRoutes, existingRoutes); err != nil {
		return ctrl.Result{}, fmt.Errorf("cleaning routes: %w", err)
	}

	if err = EnsureTablePresence(routecfg, tableID); err != nil {
		return ctrl.Result{}, fmt.Errorf("ensuring table presence: %w", err)
	}

	// existingRules/existingRoutes were snapshotted before CleanRules/CleanRoutes ran, so they no
	// longer reflect kernel state. Reusing them here is still correct: Clean only deletes entries
	// NOT in the desired spec, while Ensure*Presence only looks up entries that ARE in the desired
	// spec, so the two never operate on the same rows and the stale snapshot cannot cause a wrong
	// exists/not-exists result. Re-fetching here would just be a redundant netlink list.
	for i := range routecfg.Spec.Table.Rules {
		if err = EnsureRulePresence(&routecfg.Spec.Table.Rules[i], tableID, existingRules); err != nil {
			return ctrl.Result{}, fmt.Errorf("ensuring rule presence: %w", err)
		}
		if err := EnsureRoutesPresence(routecfg.Spec.Table.Rules[i].Routes, tableID, existingRoutes); err != nil {
			if errors.Is(err, ErrNetworkUnreachable) {
				klog.Warningf("Network is unreachable for routeconfigurationbinding %s, requeuing: %v", req.String(), err)
				return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
			}
			return ctrl.Result{}, fmt.Errorf("ensuring routes presence: %w", err)
		}
	}

	klog.Infof("Applied routeconfigurationbinding %s (routecfg %s)", req.String(), routecfg.Name)

	return ctrl.Result{}, nil
}

func (r *RouteConfigurationBindingReconciler) cleanupTable(tableID uint32) {
	existingRules, err := GetRulesByTableID(tableID)
	if err != nil {
		klog.Warningf("Unable to list rules for table %d: %v", tableID, err)
	} else {
		for i := range existingRules {
			if err := netlink.RuleDel(&existingRules[i]); err != nil {
				klog.Warningf("Unable to delete rule for table %d: %v", tableID, err)
			}
		}
	}

	existingRoutes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{Table: int(tableID)}, netlink.RT_FILTER_TABLE)
	if err != nil {
		klog.Warningf("Unable to list routes for table %d: %v", tableID, err)
	} else {
		for i := range existingRoutes {
			if err := netlink.RouteDel(&existingRoutes[i]); err != nil {
				klog.Warningf("Unable to delete route for table %d: %v", tableID, err)
			}
		}
	}

	if err := EnsureTableAbsence(tableID); err != nil {
		klog.Warningf("Unable to delete table %d: %v", tableID, err)
	}
}

// SetupWithManager registers the RouteConfigurationBindingReconciler with the manager.
// target identifies the entity running this controller (e.g. a Node or a gateway Pod);
// only RouteConfigurationBinding resources whose spec.targetRef matches are reconciled.
func (r *RouteConfigurationBindingReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager,
	target networkingv1beta1.RouteBindingTargetReference, enableRouteMonitor bool, reconcileTimeout time.Duration) error {
	klog.Infof("Starting RouteConfigurationBinding controller for target %s", target.String())

	klog.Infof("route monitor enabled: %t", enableRouteMonitor)
	src := make(chan event.GenericEvent)
	if enableRouteMonitor {
		go func() {
			utilruntime.Must(netmonitor.InterfacesMonitoring(ctx, src, &netmonitor.Options{Route: &netmonitor.OptionsRoute{Delete: true}}))
		}()
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlRouteConfigurationBinding).
		For(&networkingv1beta1.RouteConfigurationBinding{}, builder.WithPredicates(ForgeRouteTargetRefPredicate(target))).
		Watches(&networkingv1beta1.RouteConfiguration{},
			NewRouteCfgToRouteCfgBindingEnqueuer(r.Client, target)).
		WatchesRawSource(NewRouteBindingWatchSource(src, NewRouteCfgBindingEnqueuer(r.Client, target))).
		WithOptions(controller.Options{
			ReconciliationTimeout: reconcileTimeout,
		}).
		Complete(r)
}

// updateStatus updates the status of the given RouteConfigurationBinding.
func (r *RouteConfigurationBindingReconciler) updateStatus(ctx context.Context,
	routebinding *networkingv1beta1.RouteConfigurationBinding, reconcileErr error) error {
	newStatus := metav1.ConditionTrue
	if reconcileErr != nil {
		newStatus = metav1.ConditionFalse
	}

	condType := string(networkingv1beta1.RouteConfigurationBindingConditionTypeApplied)
	existing := apimeta.FindStatusCondition(routebinding.Status.Conditions, condType)
	if existing != nil && existing.Status == newStatus && existing.ObservedGeneration == routebinding.Generation {
		return nil
	}

	conditionReason := string(networkingv1beta1.RouteConfigurationBindingConditionReasonApplySucceeded)
	conditionMessage := ""
	if reconcileErr != nil {
		conditionReason = string(networkingv1beta1.RouteConfigurationBindingConditionReasonApplyFailed)
		conditionMessage = reconcileErr.Error()
	}

	apimeta.SetStatusCondition(&routebinding.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             newStatus,
		ObservedGeneration: routebinding.Generation,
		Reason:             conditionReason,
		Message:            conditionMessage,
	})

	r.EventsRecorder.Eventf(routebinding, nil, "Normal", "RouteConfigurationBindingUpdate", "Updated",
		"RouteConfigurationBinding %s/%s: %s", routebinding.Namespace, routebinding.Name, newStatus)
	if clerr := r.Client.Status().Update(ctx, routebinding); clerr != nil {
		return clerr
	}
	return nil
}

// CleanupRouteConfigurationBindings removes finalizers from any RouteConfigurationBinding
// resources pending deletion whose spec.targetRef matches the given target.
// It also deletes the corresponding netlink routes/rules/table for each binding, following the same
// approach used by the RouteConfigurationBinding controller deletion path.
// It is called after the manager has fully stopped to unblock resources that the
// reconciler did not have time to process before the pod was terminated.
func CleanupRouteConfigurationBindings(ctx context.Context, cl client.Client,
	target networkingv1beta1.RouteBindingTargetReference) {
	klog.Info("Pod stopped: cleaning up pending RouteConfigurationBinding finalizers")

	bindingList := &networkingv1beta1.RouteConfigurationBindingList{}
	if err := cl.List(ctx, bindingList); err != nil {
		klog.Errorf("Shutdown cleanup: failed to list RouteConfigurationBinding resources: %v", err)
		return
	}
	for i := range bindingList.Items {
		if MatchesRouteTargetRef(&bindingList.Items[i].Spec.TargetRef, target) {
			klog.Infof("Shutdown cleanup: processing RouteConfigurationBinding %s/%s", bindingList.Items[i].Namespace, bindingList.Items[i].Name)
			cleanupBinding(ctx, cl, &bindingList.Items[i])
		}
	}
	klog.Info("Pod stopped: completed cleanup of pending RouteConfigurationBinding finalizers")
}

func cleanupBinding(ctx context.Context, cl client.Client, binding *networkingv1beta1.RouteConfigurationBinding) {
	if binding.Status.TableName != "" {
		tableID, err := GetTableID(binding.Status.TableName)
		if err == nil {
			r := &RouteConfigurationBindingReconciler{Client: cl}
			r.cleanupTable(tableID)
			klog.Infof("Shutdown cleanup: deleted netlink configuration for RouteConfigurationBinding %s/%s",
				binding.Namespace, binding.Name)
		} else {
			klog.Warningf("Shutdown cleanup: unable to get table ID for RouteConfigurationBinding %s/%s: %v",
				binding.Namespace, binding.Name, err)
		}
	}

	ctrlutil.RemoveFinalizer(binding, RouteConfigurationBindingControllerFinalizer)
	if err := cl.Update(ctx, binding); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("Shutdown cleanup: failed to remove finalizer from RouteConfigurationBinding %s/%s: %v",
			binding.Namespace, binding.Name, err)
		return
	}
	klog.Infof("Shutdown cleanup: removed finalizer from RouteConfigurationBinding %s/%s",
		binding.Namespace, binding.Name)
}
