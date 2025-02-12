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

package fabric

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/network/geneve"
)

// InternalFabricReconciler manage internalfabric.
type InternalFabricReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
	Options        *Options
}

// NewInternalFabricReconciler returns a new InternalFabricReconciler.
func NewInternalFabricReconciler(cl client.Client, s *runtime.Scheme,
	er record.EventRecorder, opts *Options) (*InternalFabricReconciler, error) {
	return &InternalFabricReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
		Options:        opts,
	}, nil
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalfabrics,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalfabrics/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=genevetunnels,verbs=get;list;watch;update;patch

// Reconcile manage InternalFabrics.
func (r *InternalFabricReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	internalfabric := &networkingv1beta1.InternalFabric{}
	if err = r.Get(ctx, req.NamespacedName, internalfabric); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no internalfabric %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the internalfabric %q: %w", req.NamespacedName, err)
	}

	internalnode := &networkingv1beta1.InternalNode{}
	if err = r.Get(ctx, types.NamespacedName{Name: r.Options.NodeName}, internalnode); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to get the internalnode %q: %w", r.Options.NodeName, err)
	}

	klog.V(4).Infof("Reconciling internalfabric %s", req.String())

	// Manage Finalizers and routeconfiguration deletion.
	deleting := !internalfabric.ObjectMeta.DeletionTimestamp.IsZero()
	containsFinalizer := ctrlutil.ContainsFinalizer(internalfabric, internalfabricControllerFinalizer)
	switch {
	case !deleting && !containsFinalizer:
		if err = r.ensureinternalfabricFinalizerPresence(ctx, internalfabric); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil

	case deleting && containsFinalizer:
		if err := geneve.EnsureGeneveInterfaceAbsence(internalfabric.Spec.Interface.Node.Name); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to ensure the geneve interface absence: %w", err)
		}

		if err = r.ensureinternalfabricFinalizerAbsence(ctx, internalfabric); err != nil {
			return ctrl.Result{}, err
		}

		klog.V(2).Infof("InternalFabric %s deleted", req.String())

		return ctrl.Result{}, nil

	case deleting && !containsFinalizer:
		return ctrl.Result{}, nil
	}

	id, err := geneve.GetGeneveTunnelID(ctx, r.Client, internalfabric.Name, r.Options.NodeName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("internalfabriccontroller waiting for geneve tunnel creation (with id %q): %w", id, err)
	}

	if err := geneve.EnsureGeneveInterfacePresence(
		internalfabric.Spec.Interface.Node.Name,
		internalnode.Spec.Interface.Node.IP.String(),
		internalfabric.Spec.GatewayIP.String(),
		id,
		r.Options.DisableARP,
		internalfabric.Spec.MTU,
		r.Options.GenevePort,
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to ensure the geneve interface presence: %w", err)
	}

	klog.Infof("Enforced interface %s for internalfabric %s", internalfabric.Spec.Interface.Node.Name, internalfabric.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager register the InternalFabricReconciler to the manager.
func (r *InternalFabricReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlInternalFabricFabric).
		For(&networkingv1beta1.InternalFabric{}).
		Complete(r)
}
