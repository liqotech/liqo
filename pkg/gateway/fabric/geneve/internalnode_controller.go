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

package geneve

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway/fabric"
	"github.com/liqotech/liqo/pkg/utils/network/geneve"
)

// InternalNodeReconciler manage InternalNode.
type InternalNodeReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
	Options        *fabric.Options
}

// NewInternalNodeReconciler returns a new InternalNodeReconciler.
func NewInternalNodeReconciler(cl client.Client, s *runtime.Scheme,
	er record.EventRecorder, opts *fabric.Options) (*InternalNodeReconciler, error) {
	return &InternalNodeReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
		Options:        opts,
	}, nil
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalfabrics,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=genevetunnels,verbs=get;list;watch;update;patch

// Reconcile manage InternalNodes.
func (r *InternalNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	internalnode := &networkingv1beta1.InternalNode{}
	if err = r.Get(ctx, req.NamespacedName, internalnode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no internalnode %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the internalnode %q: %w", req.NamespacedName, err)
	}

	klog.V(4).Infof("Reconciling internalnode %s", req.String())

	// The internal fabric has the same name of the gateway resource.
	internalFabricName := r.Options.GwOptions.Name
	id, err := geneve.GetGeneveTunnelID(ctx, r.Client, internalFabricName, internalnode.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("internalnodecontroller waiting for geneve tunnel creation (with id %q): %w", id, err)
	}

	internalFabric := &networkingv1beta1.InternalFabric{}
	if err = r.Get(ctx, types.NamespacedName{
		Name: internalFabricName, Namespace: r.Options.GwOptions.Namespace}, internalFabric); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to get the internalfabric %q: %w", internalFabricName, err)
	}

	var remoteIP *networkingv1beta1.IP
	switch {
	case r.Options.GwOptions.NodeName == internalnode.Name:
		remoteIP = internalnode.Status.NodeIP.Local
	default:
		remoteIP = internalnode.Status.NodeIP.Remote
	}

	if remoteIP == nil {
		klog.Infof("The remote IP of internalnode %s is not set yet.", internalnode.Name)
		return ctrl.Result{RequeueAfter: time.Second * 2}, nil
	}

	if err := geneve.EnsureGeneveInterfacePresence(
		internalnode.Spec.Interface.Gateway.Name,
		internalFabric.Spec.Interface.Gateway.IP.String(),
		remoteIP.String(),
		id,
		r.Options.DisableARP,
		internalFabric.Spec.MTU,
		r.Options.GenevePort,
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to ensure the geneve interface presence: %w", err)
	}

	klog.Infof("Enforced interface %s for internalnode %s", internalnode.Spec.Interface.Gateway.Name, internalnode.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager register the InternalNodeReconciler to the manager.
func (r *InternalNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlInternalNodeGeneve).
		For(&networkingv1beta1.InternalNode{}).
		Complete(r)
}
