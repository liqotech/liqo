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

package configurationcontroller

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	configuration "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/configuration"
)

// ConfigurationReconciler manage Configuration lifecycle.
type ConfigurationReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Options *Options
}

// NewConfigurationReconciler returns a new ConfigurationReconciler.
func NewConfigurationReconciler(cl client.Client, s *runtime.Scheme, opts *Options) *ConfigurationReconciler {
	return &ConfigurationReconciler{
		Client:  cl,
		Scheme:  s,
		Options: opts,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;watch;delete;create;update;patch

// Reconcile manage Configuration lifecycle.
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	cfg := &networkingv1beta1.Configuration{}
	if err = r.Get(ctx, req.NamespacedName, cfg); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Configuration %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the Configuration %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	klog.V(4).Infof("Reconciling Configuration %q", req.NamespacedName)

	err = r.ensureFirewallConfiguration(ctx, cfg, r.Options)

	if err != nil {
		return ctrl.Result{}, err
	}

	klog.Infof("Configured masquerade bypass for Configuration %q", req.NamespacedName)

	return ctrl.Result{}, nil
}

// SetupWithManager register the ConfigurationReconciler to the manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p, err := predicate.LabelSelectorPredicate(v1.LabelSelector{
		MatchLabels: map[string]string{
			configuration.Configured: configuration.ConfiguredValue,
		},
	})
	if err != nil {
		klog.Error(err)
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlConfigurationInternal).
		For(&networkingv1beta1.Configuration{}, builder.WithPredicates(p)).
		Complete(r)
}
