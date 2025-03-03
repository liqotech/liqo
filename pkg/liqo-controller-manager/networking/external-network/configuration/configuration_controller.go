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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/cidr"
	"github.com/liqotech/liqo/pkg/utils/events"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
)

// ConfigurationReconciler manage Configuration lifecycle.
type ConfigurationReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder

	localCIDR *networkingv1beta1.ClusterConfigCIDR
}

// NewConfigurationReconciler returns a new ConfigurationReconciler.
func NewConfigurationReconciler(cl client.Client, s *runtime.Scheme, er record.EventRecorder) *ConfigurationReconciler {
	return &ConfigurationReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,

		localCIDR: nil,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations/finalizers,verbs=update
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks/status,verbs=get;list;watch

// Reconcile manage Configurations, remapping cidrs with Networks resources.
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	configuration := &networkingv1beta1.Configuration{}
	if err := r.Get(ctx, req.NamespacedName, configuration); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no configuration %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the configuration %q: %w", req.NamespacedName, err)
	}

	if configuration.Spec.Local == nil {
		if err := r.defaultLocalNetwork(ctx, configuration); err != nil {
			return ctrl.Result{}, err
		}
	}

	events.Event(r.EventsRecorder, configuration, "Processing configuration")

	if err := r.RemapConfiguration(ctx, configuration, r.EventsRecorder); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.UpdateConfigurationStatus(ctx, configuration); err != nil {
		return ctrl.Result{}, err
	}

	if !isConfigurationConfigured(configuration) {
		events.Event(r.EventsRecorder, configuration, "Waiting for all networks to be ready")
	} else {
		events.Event(r.EventsRecorder, configuration, "Configuration remapped")
		if err := SetConfigurationConfigured(ctx, r.Client, configuration); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to set configuration %q as configured: %w", req.NamespacedName, err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *ConfigurationReconciler) defaultLocalNetwork(ctx context.Context, cfg *networkingv1beta1.Configuration) error {
	if r.localCIDR == nil {
		podCIDR, err := ipamutils.GetPodCIDR(ctx, r.Client, corev1.NamespaceAll)
		if err != nil {
			return fmt.Errorf("unable to retrieve the podCIDR: %w", err)
		}

		externalCIDR, err := ipamutils.GetExternalCIDR(ctx, r.Client, corev1.NamespaceAll)
		if err != nil {
			return fmt.Errorf("unable to retrieve the externalCIDR: %w", err)
		}

		r.localCIDR = &networkingv1beta1.ClusterConfigCIDR{
			Pod:      cidr.SetPrimary(networkingv1beta1.CIDR(podCIDR)),
			External: cidr.SetPrimary(networkingv1beta1.CIDR(externalCIDR)),
		}
	}

	cfg.Spec.Local = &networkingv1beta1.ClusterConfig{
		CIDR: *r.localCIDR,
	}
	return r.Client.Update(ctx, cfg)
}

// RemapConfiguration remap the configuration using ipamv1alpha1.Network.
func (r *ConfigurationReconciler) RemapConfiguration(ctx context.Context, cfg *networkingv1beta1.Configuration,
	er record.EventRecorder) error {
	// Checks if the configuration is already remapped.
	for _, cidrType := range LabelCIDRTypeValues {
		network, err := CreateOrGetNetwork(ctx, r.Client, r.Scheme, er, cfg, cidrType)
		if err != nil {
			return fmt.Errorf("unable to create or get the network %q: %w", client.ObjectKeyFromObject(cfg), err)
		}
		if network.Status.CIDR == "" {
			continue
		}
		ForgeConfigurationStatus(cfg, network, cidrType)
	}
	return nil
}

// UpdateConfigurationStatus update the configuration.
func (r *ConfigurationReconciler) UpdateConfigurationStatus(ctx context.Context, cfg *networkingv1beta1.Configuration) error {
	if err := r.Client.Status().Update(ctx, cfg); err != nil {
		return fmt.Errorf("unable to update the status of the configuration %q: %w", client.ObjectKeyFromObject(cfg), err)
	}
	return nil
}

// ForgeConfigurationStatus create the status of the configuration.
func ForgeConfigurationStatus(cfg *networkingv1beta1.Configuration, net *ipamv1alpha1.Network, cidrType LabelCIDRTypeValue) {
	if cfg.Status.Remote == nil {
		cfg.Status.Remote = &networkingv1beta1.ClusterConfig{}
	}
	var cidrNew, cidrOld networkingv1beta1.CIDR
	cidrNew = net.Status.CIDR
	switch cidrType {
	case LabelCIDRTypePod:
		cidrOld = *cidr.GetPrimary(cfg.Spec.Remote.CIDR.Pod)
		cfg.Status.Remote.CIDR.Pod = cidr.SetPrimary(cidrNew)
	case LabelCIDRTypeExternal:
		cidrOld = *cidr.GetPrimary(cfg.Spec.Remote.CIDR.External)
		cfg.Status.Remote.CIDR.External = cidr.SetPrimary(cidrNew)
	}
	klog.Infof("Configuration %s %s CIDR: %s -> %s", client.ObjectKeyFromObject(cfg).String(), cidrType, cidrOld, cidrNew)
}

func isConfigurationConfigured(cfg *networkingv1beta1.Configuration) bool {
	if cfg.Status.Remote == nil {
		return false
	}
	return !cidr.IsVoid(cidr.GetPrimary(cfg.Status.Remote.CIDR.Pod)) && !cidr.IsVoid(cidr.GetPrimary(cfg.Status.Remote.CIDR.External))
}

// SetupWithManager register the ConfigurationReconciler to the manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlConfigurationExternal).
		For(&networkingv1beta1.Configuration{}).
		Owns(&ipamv1alpha1.Network{}).
		Complete(r)
}
