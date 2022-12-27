// Copyright 2019-2023 The Liqo Authors
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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	clientutils "github.com/liqotech/liqo/pkg/utils/clients"
)

// Reconciler reconciles a ShadowPod object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func podShouldBeUpdated(newObj, oldObj client.Object) bool {
	changesInLabels := !labels.Equals(newObj.GetLabels(), oldObj.GetLabels())
	changesInAnnots := !labels.Equals(newObj.GetAnnotations(), oldObj.GetAnnotations())

	return changesInLabels || changesInAnnots
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

	// update shadowpod labels to include the "managed-by"
	shadowPod.SetLabels(labels.Merge(shadowPod.Labels, labels.Set{consts.ManagedByLabelKey: consts.ManagedByShadowPodValue}))

	// if any existing pod is already been created from the shadowpod...
	existingPod := corev1.Pod{}
	if err := r.Get(ctx, nsName, &existingPod); err != nil && !errors.IsNotFound(err) {
		klog.Errorf("unable to get pod %s: %v", nsName, err)
		return ctrl.Result{}, err
	} else if err == nil {
		// Update Labels and Annotations
		klog.V(4).Infof("pod %q found running, will update it with labels: %v and annotations: %v",
			klog.KObj(&existingPod), existingPod.Labels, existingPod.Annotations)

		// Create Apply object for Existing Pod
		apply := corev1apply.Pod(existingPod.Name, existingPod.Namespace).
			WithLabels(shadowPod.GetLabels()).
			WithAnnotations(shadowPod.GetAnnotations())

		if err := r.Patch(ctx, &existingPod, clientutils.Patch(apply), client.ForceOwnership, client.FieldOwner("shadow-pod")); err != nil {
			klog.Errorf("unable to update pod %q: %v", klog.KObj(&existingPod), err)
			return ctrl.Result{}, err
		}

		klog.Infof("updated pod %q with success", klog.KObj(&existingPod))

		return ctrl.Result{}, nil
	}

	// ...Else, a brand new Pod must be created, based on the shadowpod.
	newPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nsName.Name,
			Namespace:   nsName.Namespace,
			Labels:      shadowPod.Labels,
			Annotations: shadowPod.Annotations,
		},
		Spec: shadowPod.Spec.Pod,
	}

	utilruntime.Must(ctrl.SetControllerReference(&shadowPod, &newPod, r.Scheme))

	if err := r.Create(ctx, &newPod, client.FieldOwner("shadow-pod")); err != nil {
		klog.Errorf("unable to create pod for shadowpod %q: %v", klog.KObj(&shadowPod), err)
		return ctrl.Result{}, err
	}

	klog.Infof("created pod %q for shadowpod %q", klog.KObj(&newPod), klog.KObj(&shadowPod))

	return ctrl.Result{}, nil
}

// SetupWithManager monitors only updates on ShadowPods.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	// Trigger a reconciliation only for Delete and Update Events.
	reconciledPredicates := predicate.Funcs{
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return podShouldBeUpdated(e.ObjectNew, e.ObjectOld) },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&vkv1alpha1.ShadowPod{}).
		Owns(&corev1.Pod{}, builder.WithPredicates(reconciledPredicates)).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}
