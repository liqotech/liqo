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

package route

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	configuration "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/configuration"
	"github.com/liqotech/liqo/pkg/utils/getters"
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
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations,verbs=get;list;watch;update;patch;create;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=ips,verbs=get;list;watch;update;patch;create;delete

// Reconcile manage InternalNodes.
func (r *InternalNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	internalnode := &networkingv1beta1.InternalNode{
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
	klog.V(4).Infof("Reconciling internalnode %s", req.String())

	InitMark(ctx, r.Client, r.Options)

	StartMarkTransaction()
	defer EndMarkTransaction()

	containsFinalizer := controllerutil.ContainsFinalizer(internalnode, internalNodesControllerFinalizer)

	// The internalnode is being deleted. Ensure the absence of RouteConfigurations and FirewallConfigurations
	// and delete internalNode finalizer.
	if !internalnode.DeletionTimestamp.IsZero() {
		if containsFinalizer {
			if err = enforceRouteWithConntrackAbsence(ctx, r.Client, internalnode, r.Options); err != nil {
				return ctrl.Result{}, err
			}

			FreeMark(internalnode.GetName())

			if err = r.enforceInternalNodeFinalizerAbsence(ctx, internalnode); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !containsFinalizer {
		if err = r.enforceInternalNodeFinalizerPresence(ctx, internalnode); err != nil {
			return ctrl.Result{}, err
		}
	}

	extCIDR, err := ipam.GetExternalCIDR(ctx, r.Client, corev1.NamespaceAll)
	if err != nil {
		return ctrl.Result{}, err
	}

	unknownSourceIP, err := ipam.GetUnknownSourceIP(extCIDR)
	if err != nil {
		return ctrl.Result{}, err
	}

	configurations, err := getters.ListConfigurationsByLabel(ctx, r.Client, labels.SelectorFromSet(labels.Set{
		configuration.Configured: configuration.ConfiguredValue,
	}))
	if err != nil {
		return ctrl.Result{}, err
	}

	ips, err := getters.ListIPsByLabel(ctx, r.Client, labels.Everything())
	if err != nil {
		return ctrl.Result{}, err
	}

	mark := AssignMark(internalnode.GetName())

	if err = enforceRouteWithConntrackPresence(ctx, r.Client, internalnode, r.Scheme, mark, unknownSourceIP, r.Options); err != nil {
		return ctrl.Result{}, err
	}

	if err := enforceRouteConfigurationExtCIDR(ctx, r.Client, internalnode, configurations.Items, ips.Items, r.Scheme, r.Options); err != nil {
		return ctrl.Result{}, err
	}

	klog.Infof("Enforced routeconfiguration and firewallconfiguration for internalnode %s", req.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager register the InternalNodeReconciler to the manager.
func (r *InternalNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlInternalNodeRoute).
		For(&networkingv1beta1.InternalNode{}).
		Watches(&networkingv1beta1.Configuration{}, handler.EnqueueRequestsFromMapFunc(r.genericEnqueuerfunc)).
		Watches(&ipamv1alpha1.IP{}, handler.EnqueueRequestsFromMapFunc(r.genericEnqueuerfunc)).
		Complete(r)
}

func (r *InternalNodeReconciler) genericEnqueuerfunc(ctx context.Context, _ client.Object) []reconcile.Request {
	internalNodes, err := getters.ListInternalNodesByLabels(ctx, r.Client, labels.Everything())
	if err != nil {
		klog.Error(err)
		return nil
	}

	var requests []reconcile.Request
	for i := range internalNodes.Items {
		iNode := &internalNodes.Items[i]

		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(iNode),
		})
	}
	return requests
}
