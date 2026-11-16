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

package ipmapping

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
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

	"github.com/liqotech/liqo/pkg/consts"
	podutils "github.com/liqotech/liqo/pkg/utils/pod"
)

// OffloadedPodReconciler manage offloaded pods lifecycle.
type OffloadedPodReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
}

// NewOffloadedPodReconciler returns a new OffloadedPodReconciler.
func NewOffloadedPodReconciler(cl client.Client, s *runtime.Scheme, er record.EventRecorder) *OffloadedPodReconciler {
	return &OffloadedPodReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
	}
}

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=ips,verbs=get;list;watch;update;patch;create;delete

// Reconcile reconciles on offloaded pods.
func (r *OffloadedPodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pod := &corev1.Pod{}
	if err := r.Client.Get(ctx, req.NamespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(6).Infof("There is no pod %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetching pod %s: %w", req.String(), err)
	}

	klog.V(4).Infof("Reconciling pod %s", req.String())

	op, err := CreateOrUpdateIP(ctx, r.Client, r.Scheme, pod)
	if err != nil {
		if errors.Is(err, podutils.ErrNoIPAddress) {
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		if errors.Is(err, podutils.ErrNoIPv4Address) {
			klog.Warningf("Skipping IP for pod %s: it has no IPv4 address (the Liqo network fabric is IPv4 only)", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("creating or updating IP for pod %s: %w", req.String(), err)
	}
	if op != controllerutil.OperationResultNone {
		klog.Infof("IP %s for pod %q", op, req.String())
	}

	return ctrl.Result{}, nil
}

// SetupWithManager monitors updates on nodes.
func (r *OffloadedPodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			consts.LocalPodLabelKey: consts.LocalPodLabelValue,
		},
	})
	if err != nil {
		return fmt.Errorf("unable to create predicate: %w", err)
	}
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlPodIPMapping).
		For(&corev1.Pod{}, builder.WithPredicates(p)).
		Complete(r)
}
