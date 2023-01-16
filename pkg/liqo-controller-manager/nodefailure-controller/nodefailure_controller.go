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

package nodefailurectrl

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
)

var nodeNameField = "spec.nodeName"

// NodeFailureReconciler reconciles a Node object.
type NodeFailureReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete

// Reconcile nodes objects.
func (r *NodeFailureReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the Node instance
	var node corev1.Node
	if err := r.Get(ctx, client.ObjectKey{Name: req.Name}, &node); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("node %s not found", req.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf("an error occurred while getting node %s: %v", req.Name, err)
		return ctrl.Result{}, err
	}

	// If node is Ready exit without doing anything
	if utils.IsNodeReady(&node) {
		return ctrl.Result{}, nil
	}

	// Node NotReady: delete all terminating pods that are managed by shadowpods
	var pods corev1.PodList
	offloadedPodSelector := client.MatchingLabelsSelector{Selector: labels.Set{consts.ManagedByLabelKey: consts.ManagedByShadowPodValue}.AsSelector()}
	nodePodSelector := client.MatchingFieldsSelector{Selector: fields.OneTermEqualSelector(nodeNameField, node.Name)}
	if err := r.List(ctx, &pods, offloadedPodSelector, nodePodSelector); err != nil {
		klog.Errorf("unable to list pods: %v", err)
		return ctrl.Result{}, err
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		if !pod.DeletionTimestamp.IsZero() {
			if err := r.Delete(ctx, pod, client.GracePeriodSeconds(0)); err != nil {
				klog.Errorf("unable to delete pod %q: %v", klog.KObj(pod), err)
				return ctrl.Result{}, err
			}
			klog.Infof("pod %q running on failed node %s deleted", klog.KObj(pod), node.Name)
		}
	}

	return ctrl.Result{}, nil
}

// getPodTerminatingEventHandler returns an event handler that reacts on Pod updates.
// In particular, it reacts on changes over pods that are terminating and managed by a ShadowPod,
// triggering the reconciliation of the related node hosting the pod.
func getPodTerminatingEventHandler() handler.EventHandler {
	return &handler.Funcs{
		UpdateFunc: func(ue event.UpdateEvent, rli workqueue.RateLimitingInterface) {
			ownedByShadowPod := ue.ObjectNew.GetLabels()[consts.ManagedByLabelKey] == consts.ManagedByShadowPodValue
			isTerminating := !ue.ObjectNew.GetDeletionTimestamp().IsZero()
			if ownedByShadowPod && isTerminating {
				pod, ok := ue.ObjectNew.(*corev1.Pod)
				if !ok {
					klog.Errorf("object %v is not a pod", ue.ObjectNew)
					return
				}
				nodeName := pod.Spec.NodeName
				if nodeName != "" {
					rli.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: nodeName}})
				}
			}
		}}
}

func extractNodeNameFromPod(rawObj client.Object) []string {
	pod, ok := rawObj.(*corev1.Pod)
	if !ok {
		return []string{}
	}
	return []string{pod.Spec.NodeName}
}

// SetupWithManager monitors updates on nodes and watch for pods that are terminating and managed by a ShadowPod.
func (r *NodeFailureReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// Add field containing node Name to the Field Indexer
	if err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.Pod{}, nodeNameField, extractNodeNameFromPod); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, getPodTerminatingEventHandler()).
		Complete(r)
}
