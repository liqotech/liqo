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

package remapping

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	configuration "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/configuration"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
)

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations,verbs=get;list;create;delete;update;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;create;delete;update;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations/status,verbs=get;update;patch

// RemappingReconciler updates the PublicKey resource used to establish the Wireguard configuration.
//
//nolint:revive // It is a standard name.
type RemappingReconciler struct {
	Client         client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
	Options        *Options
}

// NewRemappingReconciler returns a new PublicKeysReconciler.
func NewRemappingReconciler(cl client.Client, s *runtime.Scheme, er record.EventRecorder) (*RemappingReconciler, error) {
	opts, err := NewOptions()
	if err != nil {
		return nil, fmt.Errorf("unable to create the RemappingReconciler: %w", err)
	}
	return &RemappingReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
		Options:        opts,
	}, nil
}

// Reconcile manage Configuration resources.
func (r *RemappingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	conf := &networkingv1beta1.Configuration{}
	if err := r.Client.Get(ctx, req.NamespacedName, conf); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no configuration %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the configuration %q: %w", req.NamespacedName, err)
	}
	klog.V(4).Infof("Reconciling configuration %q", req.NamespacedName)

	if cidrutils.GetPrimary(conf.Spec.Remote.CIDR.Pod) != cidrutils.GetPrimary(conf.Status.Remote.CIDR.Pod) {
		if err := CreateOrUpdateNatMappingCIDR(ctx, r.Client, r.Options, conf,
			r.Scheme, PodCIDR); err != nil {
			return ctrl.Result{}, err
		}
	}

	if cidrutils.GetPrimary(conf.Spec.Remote.CIDR.External) != cidrutils.GetPrimary(conf.Status.Remote.CIDR.External) {
		if err := CreateOrUpdateNatMappingCIDR(ctx, r.Client, r.Options, conf,
			r.Scheme, ExternalCIDR); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the RemappingReconciler to the manager.
func (r *RemappingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	filterByLabelsPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			configuration.Configured: configuration.ConfiguredValue,
		},
	})
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlConfigurationRemapping).
		For(&networkingv1beta1.Configuration{}, builder.WithPredicates(filterByLabelsPredicate)).
		Complete(r)
}
