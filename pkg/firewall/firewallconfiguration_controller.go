// Copyright 2019-2023 The Liqo Authors
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
	"github.com/liqotech/liqo/pkg/utils/json"
)

// FirewallConfigurationReconciler manage Configuration lifecycle.
//
//nolint:revive // We usually the name of the reconciled resource in the controller name.
type FirewallConfigurationReconciler struct {
	NftConnection *nftables.Conn
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
}

// NewFirewallConfigurationReconciler returns a new FirewallConfigurationReconciler.
func NewFirewallConfigurationReconciler(cl client.Client, s *runtime.Scheme, er record.EventRecorder) (*FirewallConfigurationReconciler, error) {
	nftConnection, err := nftables.New()
	if err != nil {
		return nil, fmt.Errorf("unable to create nftables connection: %w", err)
	}
	return &FirewallConfigurationReconciler{
		NftConnection:  nftConnection,
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
	}, nil
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigrations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations/finalizers,verbs=update

// Reconcile manage FirewallConfigurations, applying nftables configuration.
func (r *FirewallConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	fwcfg := &networkingv1alpha1.FirewallConfiguration{}
	if err := r.Get(ctx, req.NamespacedName, fwcfg); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no firewallconfiguration %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the firewallconfiguration %q: %w", req.NamespacedName, err)
	}

	klog.V(4).Infof("Reconciling firewallconfiguration %s", req.String())

	s, err := json.Pretty(fwcfg.Spec)
	if err != nil {
		return ctrl.Result{}, err
	}
	fmt.Println(s)

	// Manage Finalizers and Table deletion.
	if fwcfg.DeletionTimestamp.IsZero() {
		if !ctrlutil.ContainsFinalizer(fwcfg, firewallConfigurationsControllerFinalizer) {
			if err := r.ensureFirewallConfigurationFinalizerPresence(ctx, fwcfg); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	} else {
		if ctrlutil.ContainsFinalizer(fwcfg, firewallConfigurationsControllerFinalizer) {
			delTable(r.NftConnection, &fwcfg.Spec.Table)
			if err := r.NftConnection.Flush(); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.ensureFirewallConfigurationFinalizerAbsence(ctx, fwcfg); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Manage table creation and update.
	addTable(r.NftConnection, &fwcfg.Spec.Table)

	return ctrl.Result{}, r.NftConnection.Flush()
}

// SetupWithManager register the FirewallConfigurationReconciler to the manager.
func (r *FirewallConfigurationReconciler) SetupWithManager(mgr ctrl.Manager, labels map[string]string) error {
	filterByLabelsPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: labels})
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.FirewallConfiguration{}, builder.WithPredicates(filterByLabelsPredicate)).
		Complete(r)
}
