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

package firewall

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/nftables"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
)

// FirewallConfigurationReconciler manage Configuration lifecycle.
//
//nolint:revive // We usually the name of the reconciled resource in the controller name.
type FirewallConfigurationReconciler struct {
	NftConnection *nftables.Conn
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
	// Labels used to filter the reconciled resources.
	Labels map[string]string
	// EnableFinalizer is used to enable the finalizer on the reconciled resources.
	EnableFinalizer bool
}

// newFirewallConfigurationReconciler returns a new FirewallConfigurationReconciler.
func newFirewallConfigurationReconciler(cl client.Client, s *runtime.Scheme,
	er record.EventRecorder, labels map[string]string, enableFinalizer bool) (*FirewallConfigurationReconciler, error) {
	nftConnection, err := nftables.New()
	if err != nil {
		return nil, fmt.Errorf("unable to create nftables connection: %w", err)
	}
	return &FirewallConfigurationReconciler{
		NftConnection:   nftConnection,
		Client:          cl,
		Scheme:          s,
		EventsRecorder:  er,
		Labels:          labels,
		EnableFinalizer: enableFinalizer,
	}, nil
}

// NewFirewallConfigurationReconcilerWithFinalizer returns a new FirewallConfigurationReconciler that uses finalizer.
func NewFirewallConfigurationReconcilerWithFinalizer(cl client.Client, s *runtime.Scheme,
	er record.EventRecorder, labels map[string]string) (*FirewallConfigurationReconciler, error) {
	return newFirewallConfigurationReconciler(cl, s, er, labels, true)
}

// NewFirewallConfigurationReconcilerWithoutFinalizer returns a new FirewallConfigurationReconciler that doesn't use finalizer.
func NewFirewallConfigurationReconcilerWithoutFinalizer(cl client.Client, s *runtime.Scheme,
	er record.EventRecorder, labels map[string]string) (*FirewallConfigurationReconciler, error) {
	return newFirewallConfigurationReconciler(cl, s, er, labels, false)
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile manage FirewallConfigurations, applying nftables configuration.
func (r *FirewallConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	fwcfg := &networkingv1alpha1.FirewallConfiguration{}
	if err = r.Get(ctx, req.NamespacedName, fwcfg); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no firewallconfiguration %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the firewallconfiguration %q: %w", req.NamespacedName, err)
	}

	klog.V(4).Infof("Reconciling firewallconfiguration %s", req.String())

	defer func() {
		err = r.UpdateStatus(ctx, r.EventsRecorder, fwcfg, err)
	}()

	// Manage Finalizers and Table deletion.
	// In nftables, table deletion automatically delete contained chains and rules.

	if fwcfg.DeletionTimestamp.IsZero() && r.EnableFinalizer {
		if !ctrlutil.ContainsFinalizer(fwcfg, firewallConfigurationsControllerFinalizer) {
			if err = r.ensureFirewallConfigurationFinalizerPresence(ctx, fwcfg); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	} else if r.EnableFinalizer {
		if ctrlutil.ContainsFinalizer(fwcfg, firewallConfigurationsControllerFinalizer) {
			delTable(r.NftConnection, &fwcfg.Spec.Table)
			if err = r.NftConnection.Flush(); err != nil {
				return ctrl.Result{}, err
			}
			if err = r.ensureFirewallConfigurationFinalizerAbsence(ctx, fwcfg); err != nil {
				return ctrl.Result{}, err
			}
			klog.V(2).Infof("FirewallConfiguration %s deleted", req.String())
		}
		return ctrl.Result{}, nil
	}

	// If table exists, it delete chains and rules which are not contained anymore in firewallconfiguration resource.
	// It also deletes chains and rules which has been updated and need to be recreated.
	if err = cleanTable(r.NftConnection, &fwcfg.Spec.Table); err != nil {
		return ctrl.Result{}, err
	}

	// We need to flush the updates to allow the recreation of updated chains/rules.
	if err = r.NftConnection.Flush(); err != nil {
		return ctrl.Result{}, err
	}

	klog.V(4).Infof("Applying firewallconfiguration %s", req.String())

	// Enforce table existence.
	table := addTable(r.NftConnection, &fwcfg.Spec.Table)

	if err = addChains(r.NftConnection, fwcfg.Spec.Table.Chains, table); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.NftConnection.Flush(); err != nil {
		return ctrl.Result{}, err
	}

	klog.Infof("Applied firewallconfiguration %s", req.String())

	return ctrl.Result{}, nil
}

// SetupWithManager register the FirewallConfigurationReconciler to the manager.
func (r *FirewallConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	klog.Infof("Starting FirewallConfiguration controller with labels %v", r.Labels)
	filterByLabelsPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: r.Labels})
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.FirewallConfiguration{}, builder.WithPredicates(filterByLabelsPredicate)).
		Complete(r)
}

// UpdateStatus updates the status of the given FirewallConfiguration.
func (r *FirewallConfigurationReconciler) UpdateStatus(ctx context.Context, er record.EventRecorder,
	fwcfg *networkingv1alpha1.FirewallConfiguration, err error) error {
	if err == nil {
		fwcfg.Status.Condition = networkingv1alpha1.FirewallConfigurationStatusConditionApplied
	} else {
		fwcfg.Status.Condition = networkingv1alpha1.FirewallConfigurationStatusConditionError
	}
	er.Eventf(fwcfg, "Normal", "FirewallConfigurationUpdate", "FirewallConfiguration: %s", fwcfg.Status.Condition)
	if clerr := r.Client.Status().Update(ctx, fwcfg); clerr != nil {
		err = errors.Join(err, clerr)
	}
	return err
}
