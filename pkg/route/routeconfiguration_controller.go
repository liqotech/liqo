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

package route

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/network/netmonitor"
)

// RouteConfigurationReconciler manage Configuration lifecycle.
//
//nolint:revive // We usually use the name of the reconciled resource in the controller name.
type RouteConfigurationReconciler struct {
	PodName string
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
	// Labels used to filter the reconciled resources.
	LabelsSets []labels.Set
	// EnableFinalizer is used to enable the finalizer on the reconciled resources.
	EnableFinalizer bool
}

// newRouteConfigurationReconciler returns a new RouteConfigurationReconciler.
func newRouteConfigurationReconciler(cl client.Client, s *runtime.Scheme, podname string,
	er record.EventRecorder, labelsSets []labels.Set, enableFinalizer bool) (*RouteConfigurationReconciler, error) {
	return &RouteConfigurationReconciler{
		PodName:         podname,
		Client:          cl,
		Scheme:          s,
		EventsRecorder:  er,
		LabelsSets:      labelsSets,
		EnableFinalizer: enableFinalizer,
	}, nil
}

// NewRouteConfigurationReconcilerWithFinalizer initializes a reconciler that uses finalizers on routeconfigurations.
func NewRouteConfigurationReconcilerWithFinalizer(cl client.Client, s *runtime.Scheme, podname string,
	er record.EventRecorder, labelsSets []labels.Set) (*RouteConfigurationReconciler, error) {
	return newRouteConfigurationReconciler(cl, s, podname, er, labelsSets, true)
}

// NewRouteConfigurationReconcilerWithoutFinalizer initializes a reconciler that doesn't use finalizers on routeconfigurations.
func NewRouteConfigurationReconcilerWithoutFinalizer(cl client.Client, s *runtime.Scheme, podname string,
	er record.EventRecorder, labelsSets []labels.Set) (*RouteConfigurationReconciler, error) {
	return newRouteConfigurationReconciler(cl, s, podname, er, labelsSets, false)
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile manage RouteConfigurations, applying nftables configuration.
func (r *RouteConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	routeconfiguration := &networkingv1beta1.RouteConfiguration{}
	if err = r.Get(ctx, req.NamespacedName, routeconfiguration); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no routeconfiguration %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the routeconfiguration %q: %w", req.NamespacedName, err)
	}

	klog.V(4).Infof("Reconciling routeconfiguration %s", req.String())

	defer func() {
		err = r.UpdateStatus(ctx, r.EventsRecorder, routeconfiguration, r.PodName, err)
	}()

	var tableID uint32
	tableID, err = GetTableID(routeconfiguration.Spec.Table.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Manage Finalizers and routeconfiguration deletion.
	deleting := !routeconfiguration.ObjectMeta.DeletionTimestamp.IsZero()
	containsFinalizer := ctrlutil.ContainsFinalizer(routeconfiguration, routeconfigurationControllerFinalizer)
	switch {
	case !deleting && !containsFinalizer && r.EnableFinalizer:
		if err = r.ensureRouteConfigurationFinalizerPresence(ctx, routeconfiguration); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil

	case deleting && containsFinalizer && r.EnableFinalizer:
		for i := range routeconfiguration.Spec.Table.Rules {
			if err = EnsureRuleAbsence(&routeconfiguration.Spec.Table.Rules[i], tableID); err != nil {
				return ctrl.Result{}, err
			}
			if err = EnsureRoutesAbsence(routeconfiguration.Spec.Table.Rules[i].Routes, tableID); err != nil {
				return ctrl.Result{}, err
			}
		}

		if err = EnsureTableAbsence(tableID); err != nil {
			return ctrl.Result{}, err
		}

		if err = r.ensureRouteConfigurationFinalizerAbsence(ctx, routeconfiguration); err != nil {
			return ctrl.Result{}, err
		}

		klog.V(2).Infof("RouteConfiguration %s deleted", req.String())

		return ctrl.Result{}, nil

	case deleting && !containsFinalizer:
		return ctrl.Result{}, nil
	}

	if err = CleanRules(routeconfiguration.Spec.Table.Rules, tableID); err != nil {
		return ctrl.Result{}, err
	}

	allRoutes := []networkingv1beta1.Route{}
	for i := range routeconfiguration.Spec.Table.Rules {
		// Append all the routes in the same table in a single array.
		// This is necessary because we can't list the route rules filtering per rule.
		allRoutes = append(allRoutes, routeconfiguration.Spec.Table.Rules[i].Routes...)
	}
	if err = CleanRoutes(allRoutes, tableID); err != nil {
		return ctrl.Result{}, err
	}

	klog.Infof("Applying routeconfiguration %s", req.String())

	if err = EnsureTablePresence(routeconfiguration, tableID); err != nil {
		return ctrl.Result{}, err
	}

	for i := range routeconfiguration.Spec.Table.Rules {
		if err = EnsureRulePresence(&routeconfiguration.Spec.Table.Rules[i], tableID); err != nil {
			return ctrl.Result{}, err
		}
		if err := EnsureRoutesPresence(routeconfiguration.Spec.Table.Rules[i].Routes, tableID); err != nil {
			return ctrl.Result{}, err
		}
	}

	klog.Infof("Applied routeconfiguration %s", req.String())

	return ctrl.Result{}, nil
}

// SetupWithManager register the RouteConfigurationReconciler to the manager.
func (r *RouteConfigurationReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	klog.Infof("Starting RouteConfiguration controller with labels %v", r.LabelsSets)

	src := make(chan event.GenericEvent)
	go func() {
		utilruntime.Must(netmonitor.InterfacesMonitoring(ctx, src, &netmonitor.Options{Route: &netmonitor.OptionsRoute{Delete: true}}))
	}()

	filterByLabelsPredicate, err := forgeLabelsPredicate(r.LabelsSets)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlRouteConfiguration).
		For(&networkingv1beta1.RouteConfiguration{}, builder.WithPredicates(filterByLabelsPredicate)).
		WatchesRawSource(NewRouteWatchSource(src, NewRouteWatchEventHandler(r.Client, r.LabelsSets))).
		Complete(r)
}

// forgeLabelsPredicate returns a predicate that filters the resources based on the given labels.
func forgeLabelsPredicate(labelsSets []labels.Set) (predicate.Predicate, error) {
	var err error
	labelPredicates := make([]predicate.Predicate, len(labelsSets))
	for i := range labelsSets {
		if labelPredicates[i], err = predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: labelsSets[i]}); err != nil {
			return nil, err
		}
	}
	return predicate.Or(labelPredicates...), nil
}

func getConditionRef(rcfg *networkingv1beta1.RouteConfiguration, podname string) *networkingv1beta1.RouteConfigurationStatusCondition {
	var conditionRef *networkingv1beta1.RouteConfigurationStatusCondition
	for i := range rcfg.Status.Conditions {
		if rcfg.Status.Conditions[i].Host == podname {
			conditionRef = &rcfg.Status.Conditions[i]
			break
		}
	}
	if conditionRef == nil {
		conditionRef = &networkingv1beta1.RouteConfigurationStatusCondition{
			Host: podname,
		}
		rcfg.Status.Conditions = append(rcfg.Status.Conditions, *conditionRef)
	}
	return conditionRef
}

// UpdateStatus updates the status of the given RouteConfiguration.
func (r *RouteConfigurationReconciler) UpdateStatus(ctx context.Context, er record.EventRecorder,
	routeconfiguration *networkingv1beta1.RouteConfiguration, podname string, err error) error {
	conditionRef := getConditionRef(routeconfiguration, podname)
	conditionRef.Host = podname
	conditionRef.Type = networkingv1beta1.RouteConfigurationStatusConditionTypeApplied

	oldStatus := conditionRef.Status
	if err == nil {
		conditionRef.Status = metav1.ConditionTrue
	} else {
		conditionRef.Status = metav1.ConditionFalse
	}
	if oldStatus != conditionRef.Status {
		conditionRef.LastTransitionTime = metav1.Now()
	}

	er.Eventf(routeconfiguration, "Normal", "RouteConfigurationUpdate", "RouteConfiguration %s: %s", conditionRef.Type, conditionRef.Status)
	if clerr := r.Client.Status().Update(ctx, routeconfiguration); clerr != nil {
		err = errors.Join(err, clerr)
	}
	return err
}
