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

package sourcedetector

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/fabric"
	"github.com/liqotech/liqo/pkg/gateway"
)

// GatewayReconciler manage gateway.
type GatewayReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
	Options        *fabric.Options
}

// NewGatewayReconciler returns a new GatewayReconciler.
func NewGatewayReconciler(cl client.Client, s *runtime.Scheme,
	er record.EventRecorder, opts *fabric.Options) (*GatewayReconciler, error) {
	return &GatewayReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
		Options:        opts,
	}, nil
}

// cluster-role
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes/status,verbs=get;list;watch;update;patch

// Reconcile manage Gateways.
func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	pod := &corev1.Pod{}
	if err = r.Get(ctx, req.NamespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no gateway pod %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the gateway pod %q: %w", req.NamespacedName, err)
	}

	internalnode := &networkingv1beta1.InternalNode{}
	if err = r.Get(ctx, client.ObjectKey{Name: r.Options.NodeName}, internalnode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Errorf("There is no internalnode %s", r.Options.NodeName)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the internal node %q: %w", r.Options.NodeName, err)
	}

	klog.V(4).Infof("Reconciling gateway pod %s", req.String())

	if pod.Status.PodIP == "" {
		klog.Infof("Gateway pod %s has no IP", req.String())
		return ctrl.Result{RequeueAfter: time.Second * 2}, nil
	}

	src, err := GetSrcIPFromDstIP(pod.Status.PodIP)
	if err != nil {
		return ctrl.Result{}, err
	}

	if pod.Spec.NodeName == r.Options.NodeName {
		internalnode.Status.NodeIP.Local = ptr.To(networkingv1beta1.IP(src))
		klog.Infof("Enforced internal node local IP %s", src)
	} else {
		internalnode.Status.NodeIP.Remote = ptr.To(networkingv1beta1.IP(src))
		klog.Infof("Enforced internal node remote IP %s", src)
	}

	return ctrl.Result{}, r.Client.Status().Update(ctx, internalnode)
}

// SetupWithManager register the GatewayReconciler to the manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	filterByLabelsGatewayPods, err := predicate.LabelSelectorPredicate(
		metav1.LabelSelector{
			MatchLabels: gateway.ForgeActiveGatewayPodLabels(),
		},
	)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlPodGateway).
		For(&corev1.Pod{}, builder.WithPredicates(filterByLabelsGatewayPods)).
		Complete(r)
}
