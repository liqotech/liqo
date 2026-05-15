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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
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

// FirewallConfigurationAttachReconciler manages FirewallConfigurationAttach lifecycle.
//
//nolint:revive // We usually use the name of the reconciled resource in the controller name.
type FirewallConfigurationAttachReconciler struct {
	NodeName      string
	NftConnection *nftables.Conn
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
	// LabelsSets are used to filter the reconciled FirewallConfigurationAttach resources.
	LabelsSets []labels.Set
}

// NewFirewallConfigurationAttachReconciler returns a new FirewallConfigurationAttachReconciler.
func NewFirewallConfigurationAttachReconciler(cl client.Client, s *runtime.Scheme, nodename string,
	er record.EventRecorder, labelsSets []labels.Set) (*FirewallConfigurationAttachReconciler, error) {
	nftConnection, err := nftables.New()
	if err != nil {
		return nil, fmt.Errorf("unable to create nftables connection: %w", err)
	}
	return &FirewallConfigurationAttachReconciler{
		NodeName:       nodename,
		NftConnection:  nftConnection,
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
		LabelsSets:     labelsSets,
	}, nil
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationattachs,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationattachs/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationattachs/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;watch

// Reconcile manages FirewallConfigurationAttaches, applying nftables configuration from the referenced FirewallConfiguration.
func (r *FirewallConfigurationAttachReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	fwattach := &networkingv1beta1.FirewallConfigurationAttach{}
	if err = r.Get(ctx, req.NamespacedName, fwattach); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(6).Infof("There is no firewallconfigurationattach %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting firewallconfigurationattach: %w", err)
	}

	klog.V(4).Infof("Reconciling firewallconfigurationattach %s", req.String())

	// Deletion path: use the table name cached in the status so the FirewallConfiguration
	// does not need to be fetched (it may already be deleted).
	if !fwattach.DeletionTimestamp.IsZero() {
		if ctrlutil.ContainsFinalizer(fwattach, firewallConfigurationAttachControllerFinalizer) {
			if fwattach.Status.TableName != "" {
				tableName := fwattach.Status.TableName
				delTable(r.NftConnection, &firewallapi.Table{Name: &tableName})
				if err = r.NftConnection.Flush(); err != nil {
					return ctrl.Result{}, fmt.Errorf("flushing nftables connection: %w", err)
				}
				klog.Infof("Deleted nftables configuration for firewallconfigurationattach %s", req.String())
			}
			if err = r.ensureAttachFinalizerAbsence(ctx, fwattach); err != nil {
				return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
			}
			klog.Infof("Removed finalizer from firewallconfigurationattach %s", req.String())
		}
		return ctrl.Result{}, nil
	}

	// Normal path: fetch the FirewallConfiguration to get the full table spec.
	fwcfg := &networkingv1beta1.FirewallConfiguration{}
	if err = r.Get(ctx, types.NamespacedName{
		Name:      fwattach.Spec.FirewallConfigurationRef.Name,
		Namespace: fwattach.Namespace,
	}, fwcfg); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("Referenced firewallconfiguration %q not found for attach %s; requeuing",
				fwattach.Spec.FirewallConfigurationRef.Name, req.String())
			return ctrl.Result{RequeueAfter: 5 * time.Second}, fmt.Errorf("referenced firewallconfiguration %q not found",
				fwattach.Spec.FirewallConfigurationRef.Name)
		}
		return ctrl.Result{}, fmt.Errorf("getting referenced firewallconfiguration: %w", err)
	}

	// Cache the table name in the status so it is available during cleanup even after
	// the FirewallConfiguration has been deleted.
	fwattach.Status.TableName = ptr.Deref(fwcfg.Spec.Table.Name, "")

	defer func() {
		err = r.updateStatus(ctx, fwattach, err)
	}()

	if !ctrlutil.ContainsFinalizer(fwattach, firewallConfigurationAttachControllerFinalizer) {
		if err = r.ensureAttachFinalizerPresence(ctx, fwattach); err != nil {
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

	klog.V(4).Infof("Applying firewallconfigurationattach %s (fwcfg %s)", req.String(), fwcfg.Name)

	table := addTable(r.NftConnection, &fwcfg.Spec.Table)
	if err = addChains(r.NftConnection, fwcfg.Spec.Table.Chains, table); err != nil {
		return ctrl.Result{}, fmt.Errorf("adding chains to table %s: %w", ptr.Deref(fwcfg.Spec.Table.Name, ""), err)
	}

	if err = r.NftConnection.Flush(); err != nil {
		return ctrl.Result{}, fmt.Errorf("flushing nftables connection: %w", err)
	}

	klog.Infof("Applied firewallconfigurationattach %s (fwcfg %s)", req.String(), fwcfg.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager registers the FirewallConfigurationAttachReconciler with the manager.
func (r *FirewallConfigurationAttachReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager,
	enableNftMonitor bool, reconcileTimeout time.Duration) error {
	klog.Infof("Starting FirewallConfigurationAttach controller with labels %v", r.LabelsSets)
	filterByLabelsPredicate, err := forgeLabelsPredicate(r.LabelsSets)
	if err != nil {
		return err
	}

	klog.Infof("nftables monitor enabled: %t", enableNftMonitor)
	src := make(chan event.GenericEvent)
	if enableNftMonitor {
		go func() {
			utilruntime.Must(netmonitor.InterfacesMonitoring(ctx, src, &netmonitor.Options{Nftables: &netmonitor.OptionsNftables{Delete: true}}))
		}()
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlFirewallConfigurationAttach).
		For(&networkingv1beta1.FirewallConfigurationAttach{}, builder.WithPredicates(filterByLabelsPredicate)).
		WatchesRawSource(NewFirewallAttachWatchSource(src, NewFirewallAttachWatchEventHandler(r.Client, r.LabelsSets))).
		WithOptions(controller.Options{
			ReconciliationTimeout: reconcileTimeout,
		}).
		Complete(r)
}

// updateStatus updates the status of the given FirewallConfigurationAttach.
func (r *FirewallConfigurationAttachReconciler) updateStatus(ctx context.Context,
	fwattach *networkingv1beta1.FirewallConfigurationAttach, reconcileErr error) error {
	fwattach.Status.Type = networkingv1beta1.FirewallConfigurationAttachConditionTypeApplied

	var newStatus metav1.ConditionStatus
	if reconcileErr == nil {
		newStatus = metav1.ConditionTrue
	} else {
		newStatus = metav1.ConditionFalse
	}

	if fwattach.Status.Status == newStatus {
		return nil
	}

	fwattach.Status.Status = newStatus
	fwattach.Status.LastTransitionTime = metav1.Now()

	r.EventsRecorder.Eventf(fwattach, "Normal", "FirewallConfigurationAttachUpdate",
		"FirewallConfigurationAttach %s/%s: %s", fwattach.Namespace, fwattach.Name, newStatus)
	if clerr := r.Client.Status().Update(ctx, fwattach); clerr != nil {
		return errors.Join(reconcileErr, clerr)
	}
	return reconcileErr
}

// forgeLabelsPredicate returns a predicate that matches resources with any of the given label sets.
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
