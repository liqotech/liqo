/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package virtualNode_controller

import (
	"context"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	constctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	virtualNodeControllerFinalizer = "virtualnode-controller.liqo.io/finalizer"
)

type VirtualNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes/status,verbs=get;update;patch

func (r *VirtualNodeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {

	node := &corev1.Node{}
	if err := r.Get(context.TODO(), req.NamespacedName, node); err != nil {
		klog.Errorf(" %s --> Unable to get virtual-node '%s'", err, req.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !ctrlutils.ContainsFinalizer(node, virtualNodeControllerFinalizer) {
		ctrlutils.AddFinalizer(node, virtualNodeControllerFinalizer)
		if err := r.Patch(context.TODO(), node, client.Merge); err != nil {
			klog.Errorf("%s --> Unable to add '%s' to the virtual node '%s'", err, virtualNodeControllerFinalizer, node.GetName())
			return ctrl.Result{}, err
		}
	}

	if !node.GetDeletionTimestamp().IsZero() {

		klog.Infof("The virtual node '%s' is requested to be deleted", node.GetName())

		nms := &mapsv1alpha1.NamespaceMapList{}
		if err := r.List(context.TODO(), nms, client.InNamespace(constctrl.MapNamespaceName),
			client.MatchingLabels{constctrl.VirtualNodeClusterId: node.GetAnnotations()[constctrl.VirtualNodeClusterId]}); err != nil {
			klog.Errorf("%s --> Unable to List NamespaceMaps of virtual node '%s'", err, node.GetName())
			return ctrl.Result{}, err
		}

		// delete all NamespaceMaps associated with this node, in normal conditions only one
		for _, nm := range nms.Items {
			if err := r.removeAllDesiredMappings(nm); err != nil {
				return ctrl.Result{}, err
			}
		}

		ctrlutils.RemoveFinalizer(node, virtualNodeControllerFinalizer)
		if err := r.Update(context.TODO(), node); err != nil {
			klog.Errorf(" %s --> Unable to remove %s from the virtual node '%s'", err, virtualNodeControllerFinalizer, node.GetName())
			return ctrl.Result{}, err
		}
		klog.Infof("Finalizer is correctly removed from the virtual node '%s'", node.GetName())
		return ctrl.Result{}, nil
	}

	if err := r.namespaceMapLifecycle(*node); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// Events not filtered:
// 1 -- creation of a new Virtual-node
// 2 -- creation of a new NamespaceMap
// 3 -- update deletionTimestamp on NamespaceMap or on Virtual-node, due to deletion request
func filterVirtualNodes() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// if the resource has no namespace, it surely a node, so we have to check if it is virtual or not, we are
			// interested only in virtual-nodes' deletion, not common nodes' deletion.
			if e.MetaNew.GetNamespace() == "" {
				if value, ok := (e.MetaNew.GetLabels())[constctrl.TypeLabel]; !ok || value != constctrl.TypeNode {
					return false
				}
			}
			// so here we monitor only NamespaceMaps' and virtual-nodes' deletion
			return !(e.MetaNew.GetDeletionTimestamp().IsZero())
		},
		CreateFunc: func(e event.CreateEvent) bool {
			// listen only virtual-node creation, not simple node
			if e.Meta.GetNamespace() == "" {
				if value, ok := (e.Meta.GetLabels())[constctrl.TypeLabel]; !ok || value != constctrl.TypeNode {
					return false
				}
			}
			// so here we monitor only NamespaceMaps' and virtual-nodes' creation
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			if e.Meta.GetNamespace() == "" {
				if value, ok := (e.Meta.GetLabels())[constctrl.TypeLabel]; !ok || value != constctrl.TypeNode {
					return false
				}
			}
			return !(e.Meta.GetDeletionTimestamp().IsZero())
		},
	}
}

func (r *VirtualNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Owns(&mapsv1alpha1.NamespaceMap{}).
		WithEventFilter(filterVirtualNodes()).
		Complete(r)
}
