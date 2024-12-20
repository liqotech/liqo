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

package ipmapping

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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	configuration "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/configuration"
	"github.com/liqotech/liqo/pkg/utils/cidr"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=get;list;watch;create;update;patch;delete

// ConfigurationReconciler creates a mapping for the UnknownSourceIP for each remote cluster.
// This allows traffic with "external" source IPs to be routed from a leaf cluster to another.
type ConfigurationReconciler struct {
	Client         client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
}

func forgeUnknownSourceIPName(cfg *networkingv1beta1.Configuration) string {
	return cfg.Name + "-unknown-source"
}

// NewConfigurationReconciler returns a new PublicKeysReconciler.
func NewConfigurationReconciler(cl client.Client, s *runtime.Scheme, er record.EventRecorder) *ConfigurationReconciler {
	return &ConfigurationReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
	}
}

// Reconcile manage Configuration resources.
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cfg := &networkingv1beta1.Configuration{}
	if err := r.Client.Get(ctx, req.NamespacedName, cfg); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no configuration %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the configuration %q: %w", req.NamespacedName, err)
	}
	klog.V(4).Infof("Reconciling configuration %q", req.NamespacedName)

	extCIDR := cidr.GetPrimary(cfg.Status.Remote.CIDR.External)
	remoteUnknownSourceIP, err := ipamutils.GetUnknownSourceIP(extCIDR.String())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to get the unknown source IP: %w", err)
	}
	if err := r.createOrUpdateUnknownSourceIPResource(ctx, cfg, remoteUnknownSourceIP); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to create or update the unknown source IP: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the RemappingReconciler to the manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	filterByLabelsPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			configuration.Configured: configuration.ConfiguredValue,
		},
	})
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlConfigurationIPMapping).
		For(&networkingv1beta1.Configuration{}, builder.WithPredicates(filterByLabelsPredicate)).
		Complete(r)
}

func (r *ConfigurationReconciler) createOrUpdateUnknownSourceIPResource(ctx context.Context,
	cfg *networkingv1beta1.Configuration, remoteUnknownSourceIP string) error {
	ip := &ipamv1alpha1.IP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      forgeUnknownSourceIPName(cfg),
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				consts.RemoteClusterID: cfg.GetName(),
			},
		},
	}
	if _, err := resource.CreateOrUpdate(ctx, r.Client, ip, func() error {
		ip.Spec = ipamv1alpha1.IPSpec{
			IP: networkingv1beta1.IP(remoteUnknownSourceIP),
		}
		return controllerutil.SetOwnerReference(cfg, ip, r.Scheme)
	}); err != nil {
		return fmt.Errorf("unable to create or update the IP %q: %w", ip.Name, err)
	}
	return nil
}
