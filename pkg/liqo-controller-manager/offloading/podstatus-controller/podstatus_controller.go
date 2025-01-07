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

package podstatusctrl

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/indexer"
)

// HasRemoteUnavailableLabel return true if the pod has the remote unavailable label.
func HasRemoteUnavailableLabel(pod *corev1.Pod) bool {
	value, ok := pod.Labels[consts.RemoteUnavailableKey]
	return ok && value == consts.RemoteUnavailableValue
}

// PodStatusReconciler reconciles Liqo nodes.
type PodStatusReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles on Liqo nodes and manages the labels of the pods scheduled on them.
func (r *PodStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the Node instance
	var node corev1.Node
	if err := r.Client.Get(ctx, client.ObjectKey{Name: req.Name}, &node); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("node %s not found", req.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf("an error occurred while getting node %s: %v", req.Name, err)
		return ctrl.Result{}, err
	}

	// Skip reconciliation if it is not a Liqo virtual node
	if !utils.IsVirtualNode(&node) {
		klog.V(6).Infof("Skipping reconciliation: node %q is not a Liqo virtual node", req.Name)
		return ctrl.Result{}, nil
	}

	klog.V(4).Infof("reconciliation on liqo virtual node: %q", req.Name)

	// List all pods that are local and offloaded (running in another cluster), and scheduled on the reconciled liqo node
	var pods corev1.PodList
	localOffloadedPodSelector := client.MatchingLabels{consts.LocalPodLabelKey: consts.LocalPodLabelValue}
	nodeSelector := client.MatchingFields{indexer.FieldNodeNameFromPod: node.Name}
	if err := r.List(ctx, &pods, localOffloadedPodSelector, nodeSelector); err != nil {
		klog.Errorf("unable to list pods: %v", err)
		return ctrl.Result{}, err
	}

	// Check node status
	nodeReady := utils.IsNodeReady(&node)

	// Enforce the presence of the pod remote unavailable label based on the current node readiness status
	for i := range pods.Items {
		pod := &pods.Items[i]

		switch {
		case !nodeReady && !HasRemoteUnavailableLabel(pod):
			// Ensure the presence of the remote unavailable label
			if pod.Labels == nil {
				pod.Labels = map[string]string{}
			}
			pod.Labels[consts.RemoteUnavailableKey] = consts.RemoteUnavailableValue
			if err := r.Update(ctx, pod); err != nil {
				klog.Errorf("failed to update pod %q: %v", klog.KObj(pod), err)
				return ctrl.Result{}, err
			}
			klog.Infof("Added label %q to pod %q running on NOT ready node %s", consts.RemoteUnavailableKey, klog.KObj(pod), node.Name)

		case nodeReady && HasRemoteUnavailableLabel(pod):
			// Ensure the absence of the remote unavailable label
			delete(pod.Labels, consts.RemoteUnavailableKey)
			if err := r.Update(ctx, pod); err != nil {
				klog.Errorf("failed to update pod %q: %v", klog.KObj(pod), err)
				return ctrl.Result{}, err
			}
			klog.Infof("Removed label %q to pod %q running on ready node %s", consts.RemoteUnavailableKey, klog.KObj(pod), node.Name)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager monitors updates on nodes.
func (r *PodStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlPodStatus).
		For(&corev1.Node{}).
		Complete(r)
}
