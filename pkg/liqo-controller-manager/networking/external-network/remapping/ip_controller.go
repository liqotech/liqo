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
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

// IPReconciler manage IP.
type IPReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewIPReconciler returns a new IPReconciler.
func NewIPReconciler(cl client.Client, s *runtime.Scheme) *IPReconciler {
	return &IPReconciler{
		Client: cl,
		Scheme: s,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=create;get;list;watch;update;patch

// Reconcile manage IPs.
func (r *IPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	ip := &ipamv1alpha1.IP{}
	if err = r.Get(ctx, req.NamespacedName, ip); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no IP %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the IP %q: %w", req.NamespacedName, err)
	}

	klog.V(4).Infof("Reconciling IP %s", req.String())

	deleting := !ip.DeletionTimestamp.IsZero()
	containsFinalizer := controllerutil.ContainsFinalizer(ip, ipMappingControllerFinalizer)

	if deleting {
		if containsFinalizer {
			if err := DeleteNatMappingIP(ctx, r.Client, ip); err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to delete the NAT mapping for the IP %q: %w", req.NamespacedName, err)
			}
			if err := r.ensureIPMappingFinalizerAbsence(ctx, ip); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, nil
	}

	if ip.Status.IP == "" {
		klog.Warningf("IP %q has no IP assigned yet", req.String())
		return ctrl.Result{}, nil
	}

	if !containsFinalizer {
		if err := r.ensureIPMappingFinalizerPresence(ctx, ip); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := CreateOrUpdateNatMappingIP(ctx, r.Client, ip); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to create or update the NAT mapping for the IP %q: %w", req.NamespacedName, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the IPReconciler to the manager.
func (r *IPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	filterByLabelsPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			consts.IPHostUnreachableKey: consts.IPHostUnreachableValue,
		},
	})
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlIPRemapping).
		For(&ipamv1alpha1.IP{}, builder.WithPredicates(predicate.Not(filterByLabelsPredicate))).
		Complete(r)
}
