// Copyright 2019-2022 The Liqo Authors
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

package shadowpodctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	liqoerrors "github.com/liqotech/liqo/pkg/utils/errors"
)

// Reconciler reconciles a ShadowPod object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=shadowpods,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=shadowpods/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile ShadowPods objects.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	nsName := req.NamespacedName
	klog.V(4).Infof("reconcile shadowpod %s", nsName)

	shadowPod := vkv1alpha1.ShadowPod{}
	if err := r.Get(ctx, nsName, &shadowPod); err != nil {
		err = client.IgnoreNotFound(err)
		if err == nil {
			klog.V(4).Infof("skip: shadowpod %s not found", nsName)
		}
		return ctrl.Result{}, err
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nsName.Name,
			Namespace:   nsName.Namespace,
			Labels:      labels.Merge(shadowPod.Labels, labels.Set{consts.ManagedByLabelKey: consts.ManagedByShadowPodValue}),
			Annotations: shadowPod.Annotations,
		},
		Spec: shadowPod.Spec.Pod,
	}

	utilruntime.Must(ctrl.SetControllerReference(&shadowPod, &pod, r.Scheme))

	if err := r.Get(ctx, nsName, &pod); err == nil {
		klog.V(4).Infof("skip: pod %q already running", klog.KObj(&pod))
		return ctrl.Result{}, nil
	}

	if err := r.Create(ctx, &pod); err != nil {
		err = fmt.Errorf("unable to create pod for shadowpod %q: %w", klog.KObj(&shadowPod), err)
		return ctrl.Result{}, liqoerrors.IgnoreAlreadyExists(err)
	}

	klog.Infof("created pod %q for shadowpod %q", klog.KObj(&pod), klog.KObj(&shadowPod))

	return ctrl.Result{}, nil
}

// SetupWithManager monitors only updates on ShadowPods.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	// Trigger a reconciliation only for DeleteEvent.
	deletedPredicate := predicate.Funcs{
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&vkv1alpha1.ShadowPod{}).
		Owns(&corev1.Pod{}, builder.WithPredicates(deletedPredicate)).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}
