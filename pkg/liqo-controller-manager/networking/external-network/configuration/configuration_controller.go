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

package configurationcontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	networkingutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/utils"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
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
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks/status,verbs=get;list;watch

// Reconcile manage Configurations, remapping cidrs with Networks resources.
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	configuration := &networkingv1beta1.Configuration{}
	if err := r.Get(ctx, req.NamespacedName, configuration); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(6).Infof("There is no configuration %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the configuration %q: %w", req.NamespacedName, err)
	}

	originalCfg := configuration.DeepCopy()
	if configuration.Spec.Local == nil {
		if err := r.defaultLocalNetwork(ctx, configuration); err != nil {
			return ctrl.Result{}, err
		}
	}

	events.Event(r.EventsRecorder, configuration, "Processing configuration")

	if err := r.RemapConfiguration(ctx, configuration, r.EventsRecorder); err != nil {
		return ctrl.Result{}, err
	}

	// Stamp ObservedGeneration only when the remap is fully complete; otherwise leave it at the
	// previous value so downstream consumers see "still stale" until reconcile catches up.
	if isRemapComplete(configuration) {
		configuration.Status.ObservedGeneration = configuration.Generation
	}

	if !equality.Semantic.DeepEqual(originalCfg, configuration) {
		if err := r.UpdateConfigurationStatus(ctx, configuration); err != nil {
			return ctrl.Result{}, err
		}

		klog.Infof("Configuration %s status updated", req.NamespacedName)

		if networkingutils.IsConfigurationObserved(configuration) {
			events.Event(r.EventsRecorder, configuration, "Configuration remapped")
		} else {
			events.Event(r.EventsRecorder, configuration, "Waiting for all networks to be ready")
		}
	}

	return ctrl.Result{}, nil
}

// isRemapComplete reports whether RemapConfiguration has populated the full status arrays for
// the current spec: lengths match and no positions are empty. It does NOT check generation
// parity — that is the job that this function gates.
func isRemapComplete(cfg *networkingv1beta1.Configuration) bool {
	if cfg.Status.Remote == nil {
		return false
	}
	if len(cfg.Status.Remote.CIDR.Pod) != len(cfg.Spec.Remote.CIDR.Pod) ||
		len(cfg.Status.Remote.CIDR.External) != len(cfg.Spec.Remote.CIDR.External) {
		return false
	}
	return cidrutils.AllNonVoid(cfg.Status.Remote.CIDR.Pod) &&
		cidrutils.AllNonVoid(cfg.Status.Remote.CIDR.External)
}

func (r *ConfigurationReconciler) defaultLocalNetwork(ctx context.Context, cfg *networkingv1beta1.Configuration) error {
	if r.localCIDR == nil {
		podCIDRs, err := ipamutils.GetPodCIDRs(ctx, r.Client, corev1.NamespaceAll)
		if err != nil {
			return fmt.Errorf("unable to retrieve the podCIDR: %w", err)
		}

		externalCIDRs, err := ipamutils.GetExternalCIDRs(ctx, r.Client, corev1.NamespaceAll)
		if err != nil {
			return fmt.Errorf("unable to retrieve the externalCIDR: %w", err)
		}

		r.localCIDR = &networkingv1beta1.ClusterConfigCIDR{
			Pod:      cidrutils.FromStrings(podCIDRs),
			External: cidrutils.FromStrings(externalCIDRs),
		}
	}

	cfg.Spec.Local = &networkingv1beta1.ClusterConfig{
		CIDR: *r.localCIDR,
	}
	return r.Client.Update(ctx, cfg)
}

// RemapConfiguration ensures one ipamv1alpha1.Network resource per spec CIDR (per cidr-type),
// deletes Networks for CIDRs no longer in the spec, and populates the configuration status with
// the IPAM-remapped values, index-aligned with the spec. Positions whose corresponding Network
// has not yet been remapped by the IPAM are left as empty CIDRs.
func (r *ConfigurationReconciler) RemapConfiguration(ctx context.Context, cfg *networkingv1beta1.Configuration,
	er record.EventRecorder) error {
	for _, cidrType := range LabelCIDRTypeValues {
		specCIDRs := selectSpecCIDRs(cfg, cidrType)

		if err := DeleteOrphanNetworks(ctx, r.Client, cfg, cidrType, specCIDRs); err != nil {
			return fmt.Errorf("unable to delete orphan networks for cidr-type %q: %w", cidrType, err)
		}

		remapped := make([]networkingv1beta1.CIDR, len(specCIDRs))
		for i, c := range specCIDRs {
			nw, err := EnsureNetwork(ctx, r.Client, r.Scheme, er, cfg, cidrType, c)
			if err != nil {
				return fmt.Errorf("unable to ensure network for CIDR %q: %w", c, err)
			}
			remapped[i] = nw.Status.CIDR
		}
		writeStatusForCIDRType(cfg, cidrType, remapped)
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

func selectSpecCIDRs(cfg *networkingv1beta1.Configuration, cidrType LabelCIDRTypeValue) []networkingv1beta1.CIDR {
	switch cidrType {
	case LabelCIDRTypePod:
		return cfg.Spec.Remote.CIDR.Pod
	case LabelCIDRTypeExternal:
		return cfg.Spec.Remote.CIDR.External
	}
	return nil
}

func writeStatusForCIDRType(cfg *networkingv1beta1.Configuration, cidrType LabelCIDRTypeValue, remapped []networkingv1beta1.CIDR) {
	if cfg.Status.Remote == nil {
		cfg.Status.Remote = &networkingv1beta1.ClusterConfig{}
	}
	switch cidrType {
	case LabelCIDRTypePod:
		cfg.Status.Remote.CIDR.Pod = remapped
	case LabelCIDRTypeExternal:
		cfg.Status.Remote.CIDR.External = remapped
	}
}

// SetupWithManager register the ConfigurationReconciler to the manager.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlConfigurationExternal).
		For(&networkingv1beta1.Configuration{}).
		Owns(&ipamv1alpha1.Network{}).
		Complete(r)
}
