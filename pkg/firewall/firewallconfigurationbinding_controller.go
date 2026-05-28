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

package firewall

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/nftables"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/network/netmonitor"
)

// FirewallConfigurationBindingReconciler manages FirewallConfigurationBinding lifecycle.
//
//nolint:revive // We usually use the name of the reconciled resource in the controller name.
type FirewallConfigurationBindingReconciler struct {
	NodeName      string
	NftConnection *nftables.Conn
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder events.EventRecorder
}

// NewFirewallConfigurationBindingReconciler returns a new FirewallConfigurationBindingReconciler.
func NewFirewallConfigurationBindingReconciler(cl client.Client, s *runtime.Scheme, nodename string,
	er events.EventRecorder) (*FirewallConfigurationBindingReconciler, error) {
	nftConnection, err := nftables.New()
	if err != nil {
		return nil, fmt.Errorf("unable to create nftables connection: %w", err)
	}
	return &FirewallConfigurationBindingReconciler{
		NodeName:       nodename,
		NftConnection:  nftConnection,
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
	}, nil
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;watch

// Reconcile manages FirewallConfigurationBindinges, applying nftables configuration from the referenced FirewallConfiguration.
func (r *FirewallConfigurationBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	fwbinding := &networkingv1beta1.FirewallConfigurationBinding{}
	if err = r.Get(ctx, req.NamespacedName, fwbinding); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(6).Infof("There is no firewallconfigurationbinding %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting firewallconfigurationbinding: %w", err)
	}

	klog.V(4).Infof("Reconciling firewallconfigurationbinding %s", req.String())

	// Deletion path: use the table name cached in the status so the FirewallConfiguration
	// does not need to be fetched (it may already be deleted).
	if !fwbinding.DeletionTimestamp.IsZero() {
		if ctrlutil.ContainsFinalizer(fwbinding, firewallConfigurationBindingControllerFinalizer) {
			if fwbinding.Status.TableName != "" {
				tableName := fwbinding.Status.TableName
				delTable(r.NftConnection, &firewallapi.Table{Name: &tableName})
				if err = r.NftConnection.Flush(); err != nil {
					return ctrl.Result{}, fmt.Errorf("flushing nftables connection: %w", err)
				}
				klog.Infof("Deleted nftables configuration for firewallconfigurationbinding %s", req.String())
			}
			if err = r.ensureBindingFinalizerAbsence(ctx, fwbinding); err != nil {
				return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
			}
			klog.Infof("Removed finalizer from firewallconfigurationbinding %s", req.String())
		}
		return ctrl.Result{}, nil
	}

	// Normal path: fetch the FirewallConfiguration to get the full table spec.
	fwcfg := &networkingv1beta1.FirewallConfiguration{}
	if err = r.Get(ctx, types.NamespacedName{
		Name:      fwbinding.Spec.FirewallConfigurationRef.Name,
		Namespace: fwbinding.Namespace,
	}, fwcfg); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("Referenced firewallconfiguration %q not found for binding %s; requeuing",
				fwbinding.Spec.FirewallConfigurationRef.Name, req.String())
			return ctrl.Result{RequeueAfter: 5 * time.Second}, fmt.Errorf("referenced firewallconfiguration %q not found",
				fwbinding.Spec.FirewallConfigurationRef.Name)
		}
		return ctrl.Result{}, fmt.Errorf("getting referenced firewallconfiguration: %w", err)
	}

	// Cache the table name in the status so it is available during cleanup even after
	// the FirewallConfiguration has been deleted.
	fwbinding.Status.TableName = ptr.Deref(fwcfg.Spec.Table.Name, "")

	defer func() {
		err = r.updateStatus(ctx, fwbinding, err)
	}()

	if !ctrlutil.ContainsFinalizer(fwbinding, firewallConfigurationBindingControllerFinalizer) {
		if err = r.ensureBindingFinalizerPresence(ctx, fwbinding); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// Remove outdated chains/rules that are no longer in the spec, and flush deletions before adding new ones.
	if err = cleanTable(r.NftConnection, &fwcfg.Spec.Table); err != nil {
		return ctrl.Result{}, fmt.Errorf("cleaning table %s: %w", ptr.Deref(fwcfg.Spec.Table.Name, ""), err)
	}

	if err = r.NftConnection.Flush(); err != nil {
		return ctrl.Result{}, fmt.Errorf("flushing nftables connection: %w", err)
	}

	klog.V(4).Infof("Applying firewallconfigurationbinding %s (fwcfg %s)", req.String(), fwcfg.Name)

	table := addTable(r.NftConnection, &fwcfg.Spec.Table)
	if err = addChains(r.NftConnection, fwcfg.Spec.Table.Chains, table); err != nil {
		return ctrl.Result{}, fmt.Errorf("adding chains to table %s: %w", ptr.Deref(fwcfg.Spec.Table.Name, ""), err)
	}

	if err = r.NftConnection.Flush(); err != nil {
		return ctrl.Result{}, fmt.Errorf("flushing nftables connection: %w", err)
	}

	klog.Infof("Applied firewallconfigurationbinding %s (fwcfg %s)", req.String(), fwcfg.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager registers the FirewallConfigurationBindingReconciler with the manager.
// targetID is the identifier of the entity running this controller (e.g. fabric node name or gateway name);
// only FirewallConfigurationBinding resources whose spec.targetID matches are reconciled.
func (r *FirewallConfigurationBindingReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager,
	targetID string, enableNftMonitor bool, reconcileTimeout time.Duration) error {
	klog.Infof("Starting FirewallConfigurationBinding controller for targetID %q", targetID)

	klog.Infof("nftables monitor enabled: %t", enableNftMonitor)
	src := make(chan event.GenericEvent)
	if enableNftMonitor {
		go func() {
			utilruntime.Must(netmonitor.InterfacesMonitoring(ctx, src, &netmonitor.Options{Nftables: &netmonitor.OptionsNftables{Delete: true}}))
		}()
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlFirewallConfigurationBinding).
		For(&networkingv1beta1.FirewallConfigurationBinding{}, builder.WithPredicates(forgeTargetIDPredicate(targetID))).
		WatchesRawSource(NewFirewallBindingWatchSource(src, NewFirewallBindingWatchEventHandler(r.Client, targetID))).
		WithOptions(controller.Options{
			ReconciliationTimeout: reconcileTimeout,
		}).
		Complete(r)
}

// updateStatus updates the status of the given FirewallConfigurationBinding.
func (r *FirewallConfigurationBindingReconciler) updateStatus(ctx context.Context,
	fwbinding *networkingv1beta1.FirewallConfigurationBinding, reconcileErr error) error {
	fwbinding.Status.Type = networkingv1beta1.FirewallConfigurationBindingConditionTypeApplied

	var newStatus metav1.ConditionStatus
	if reconcileErr == nil {
		newStatus = metav1.ConditionTrue
	} else {
		newStatus = metav1.ConditionFalse
	}

	if fwbinding.Status.Status == newStatus {
		return nil
	}

	fwbinding.Status.Status = newStatus
	fwbinding.Status.LastTransitionTime = metav1.Now()

	r.EventsRecorder.Eventf(fwbinding, nil, "Normal", "FirewallConfigurationBindingUpdate", "Updated",
		"FirewallConfigurationBinding %s/%s: %s", fwbinding.Namespace, fwbinding.Name, newStatus)
	if clerr := r.Client.Status().Update(ctx, fwbinding); clerr != nil {
		return errors.Join(reconcileErr, clerr)
	}
	return reconcileErr
}

// forgeTargetIDPredicate returns a predicate that matches FirewallConfigurationBinding resources
// whose spec.targetID equals the given targetID.
func forgeTargetIDPredicate(targetID string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		binding, ok := obj.(*networkingv1beta1.FirewallConfigurationBinding)
		if !ok {
			return false
		}
		return binding.Spec.TargetID == targetID
	})
}
