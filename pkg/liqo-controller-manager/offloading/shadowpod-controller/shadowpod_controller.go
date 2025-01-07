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

package shadowpodctrl

import (
	"context"
	"slices"

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

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	clientutils "github.com/liqotech/liqo/pkg/utils/clients"
	ipamips "github.com/liqotech/liqo/pkg/utils/ipam/mapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// Reconciler reconciles a ShadowPod object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=offloading.liqo.io,resources=shadowpods,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=shadowpods/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=shadowpods/status,verbs=get;update;patch

// Reconcile ShadowPods objects.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	nsName := req.NamespacedName
	klog.V(4).Infof("reconcile shadowpod %s", nsName)
	shadowPod := offloadingv1beta1.ShadowPod{}
	if err := r.Get(ctx, nsName, &shadowPod); err != nil {
		err = client.IgnoreNotFound(err)
		if err == nil {
			klog.V(4).Infof("skip: shadowpod %s not found", nsName)
		}
		return ctrl.Result{}, err
	}

	if shadowPod.Spec.Pod.RestartPolicy == corev1.RestartPolicyNever &&
		(shadowPod.Status.Phase == corev1.PodSucceeded || shadowPod.Status.Phase == corev1.PodFailed) {
		klog.V(4).Infof("skip: shadowpod %s already succeeded or failed and restart policy set to Never", shadowPod.GetName())
		return ctrl.Result{}, nil
	} else if shadowPod.Spec.Pod.RestartPolicy == corev1.RestartPolicyOnFailure && shadowPod.Status.Phase == corev1.PodSucceeded {
		klog.V(4).Infof("skip: shadowpod %s already succeeded and restart policy set to OnFailure", shadowPod.GetName())
		return ctrl.Result{}, nil
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
		klog.V(4).Infof("pod %q found in cluster, will update it with labels: %v and annotations: %v",
			klog.KObj(&existingPod), existingPod.Labels, existingPod.Annotations)

		// Create Apply object for Existing Pod
		apply := corev1apply.Pod(existingPod.Name, existingPod.Namespace).
			WithLabels(shadowPod.GetLabels()).
			WithAnnotations(shadowPod.GetAnnotations())

		if err := r.Patch(ctx, &existingPod, clientutils.Patch(apply), client.ForceOwnership, client.FieldOwner("shadow-pod")); err != nil {
			klog.Errorf("unable to update pod %q: %v", klog.KObj(&existingPod), err)
			return ctrl.Result{}, err
		}

		// Update ShadowPod status same as Pod status
		shadowPod.Status.Phase = existingPod.Status.DeepCopy().Phase
		if newErr := r.Client.Status().Update(ctx, &shadowPod); newErr != nil {
			klog.Error(newErr)
			return ctrl.Result{}, newErr
		}

		klog.Infof("updated pod %q with success", klog.KObj(&existingPod))

		return ctrl.Result{}, nil
	}

	remoteClusterID, ok := utils.GetClusterIDFromLabelsWithKey(shadowPod.Labels, forge.LiqoOriginClusterIDKey)
	if !ok {
		klog.Errorf("unable to get remote cluster ID from shadowpod %q", klog.KObj(&shadowPod))
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

	// Mutate PodSpec
	if err := r.mutatePodSpec(ctx, &newPod.Spec, remoteClusterID); err != nil {
		klog.Errorf("unable to mutate pod spec for shadowpod %q: %v", klog.KObj(&shadowPod), err)
		return ctrl.Result{}, err
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
		DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
		CreateFunc:  func(_ event.CreateEvent) bool { return false },
		UpdateFunc:  func(_ event.UpdateEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlShadowPod).
		For(&offloadingv1beta1.ShadowPod{}).
		Owns(&corev1.Pod{}, builder.WithPredicates(reconciledPredicates)).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}

func (r *Reconciler) mutatePodSpec(ctx context.Context,
	podSpec *corev1.PodSpec, remoteClusterID liqov1beta1.ClusterID) error {
	if len(podSpec.HostAliases) == 0 {
		return nil
	}

	for i := range podSpec.HostAliases {
		if !slices.Contains(podSpec.HostAliases[i].Hostnames, forge.KubernetesAPIService) {
			continue
		}

		// If the HostAliases contains the kubernetes service hostname, it must be replaced with the remapped IP.

		ip := podSpec.HostAliases[i].IP

		// Get the remapped IP for the Kubernetes service.
		rIP, err := ipamips.MapAddress(ctx, r.Client, remoteClusterID, ip)
		if err != nil {
			return err
		}

		// Update the HostAliases with the remapped IP.
		podSpec.HostAliases[i].IP = rIP

		return nil
	}

	return nil
}
