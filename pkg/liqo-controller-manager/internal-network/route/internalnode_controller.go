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

package route

import (
	"context"
	"fmt"

	"github.com/google/nftables"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/ipam"
)

// InternalNodeReconciler manage InternalNode.
type InternalNodeReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
	Options        *Options
}

// NewInternalNodeReconciler returns a new InternalNodeReconciler.
func NewInternalNodeReconciler(cl client.Client, s *runtime.Scheme,
	er record.EventRecorder, options *Options) *InternalNodeReconciler {
	return &InternalNodeReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
		Options:        options,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations,verbs=get;list;watch;update;patch;create;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;watch;update;patch;create;delete

// Reconcile manage InternalNodes.
func (r *InternalNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	internalnode := &networkingv1alpha1.InternalNode{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}
	if err = r.Get(ctx, req.NamespacedName, internalnode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no internalnode %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the internalnode %q: %w", req.NamespacedName, err)
	}

	InitMark(ctx, r.Client, r.Options)

	klog.V(4).Infof("Reconciling internalnode %s", req.String())

	podCIDR, err := ipam.GetPodCIDR(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	firstIP, _, err := nftables.NetFirstAndLastIP(podCIDR)
	if err != nil {
		return ctrl.Result{}, err
	}

	StartMarkTransaction()
	defer EndMarkTransaction()
	mark := AssignMark(internalnode.GetName())

	containsFinalizer := controllerutil.ContainsFinalizer(internalnode, internalNodesControllerFinalizer)
	if internalnode.DeletionTimestamp.IsZero() {
		if !containsFinalizer {
			if err = r.enforceInternalNodeFinalizerPresence(ctx, internalnode); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if containsFinalizer {
			if err = enforceRouteWithConntrackAbsence(ctx, r.Client, internalnode, r.Options); err != nil {
				return ctrl.Result{}, err
			}
			FreeMark(internalnode.GetName())
			if err = r.enforceInternalNodeFinalizerAbsence(ctx, internalnode); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	klog.Infof("Assigning mark %d to internalnode %s", mark, req.Name)

	if err = enforceRouteWithConntrackPresence(ctx, r.Client, internalnode, r.Scheme, mark, firstIP.String(), r.Options); err != nil {
		return ctrl.Result{}, err
	}

	klog.Infof("Enforced routeconfiguration and firewallconfiguration for internalnode %s", req.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager register the InternalNodeReconciler to the manager.
func (r *InternalNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.InternalNode{}).
		Complete(r)
}
