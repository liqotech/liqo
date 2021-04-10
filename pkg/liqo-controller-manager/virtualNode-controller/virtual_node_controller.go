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
	namespaceresourcesv1 "github.com/liqotech/liqo/apis/virtualKubelet/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	clusterId             = "cluster-id"
	mapNamespaceName      = "default" // TODO: define in which namespace namespaceMaps must be created
	namespaceMapFinalizer = "namespacemap.liqo.io/finalizer"
	virtualNodeFinalizer  = "virtualnode.liqo.io/finalizer"
)

type VirtualNodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func (r *VirtualNodeReconciler) setOwner(nm *namespaceresourcesv1.NamespaceMap, n *corev1.Node) {
	nm.SetOwnerReferences(append(nm.OwnerReferences, metav1.OwnerReference{
		APIVersion:         "v1",
		BlockOwnerDeletion: pointer.BoolPtr(true),
		Kind:               "Node",
		Name:               n.GetName(),
		UID:                n.GetUID(),
		Controller:         pointer.BoolPtr(true), // without this don't know which controller must be called on deletion
	}))
}

func (r *VirtualNodeReconciler) namespaceMapDeletionLogic(nm *namespaceresourcesv1.NamespaceMap) error {
	if containsString(nm.GetFinalizers(), namespaceMapFinalizer) {
		// TODO: decide if recreate namespaceMap or delete all remote namespaces and terminate, or deny deletion
		// ----- caso 1: posso estrarre le nattingTable dallo stato e ricrearle
		//nt := nm.Status.NattingTable
		//dnt := nm.Status.DeNattingTable

		// ----- caso 2: potrei anche togliere il deletionTimestamp e non far cancellare la risorsa
		//nm.DeletionTimestamp = nil

		// ----- caso 3: cancellazione di tutti i namespace remoti e ricreazione della NamespaceMap vuota
		//               in teoria solo cluster admin dovrebbe poter cancellare la risorsa

		klog.Infof("Someone try to delete namespaceMap, ok delete!!")
		nm.SetFinalizers(removeString(nm.GetFinalizers(), namespaceMapFinalizer))

		// TODO: Update to patch.apply()
		if err := r.Update(context.TODO(), nm); err != nil {
			klog.Errorln(err, " -------------- Problems while removing NamespaceMap finalizer --------------")
			return err
		}
	}
	return nil
}

func (r *VirtualNodeReconciler) namespaceMapLifecycle(nm *namespaceresourcesv1.NamespaceMap, n *corev1.Node) error {

	if err := r.Get(context.TODO(), types.NamespacedName{Namespace: mapNamespaceName, Name: n.GetAnnotations()[clusterId]}, nm); err != nil {
		if errors.IsNotFound(err) {

			klog.Infof(" create NamespaceMap for " + n.Name + "\n")
			nm = &namespaceresourcesv1.NamespaceMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "namespaceresources.liqo.io/v1",
					Kind:       "NamespaceMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      n.GetAnnotations()[clusterId],
					Namespace: mapNamespaceName,
				},
				Spec: namespaceresourcesv1.NamespaceMapSpec{
					RemoteClusterId: n.GetAnnotations()[clusterId],
				},
			}

			nm.SetFinalizers(append(nm.GetFinalizers(), namespaceMapFinalizer))
			r.setOwner(nm, n)

			if err = r.Create(context.TODO(), nm); err != nil {
				klog.Errorln(err, " -------------- Problems in NamespaceMap creation --------------")
				return err
			}
		} else {
			klog.Errorln(err, " -------------- Unable to get NamespaceMap --------------")
			return err
		}
	}

	// if true namespaceMap is being deleted
	if !nm.GetDeletionTimestamp().IsZero() {
		return r.namespaceMapDeletionLogic(nm)
	}

	return nil
}

// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes/status,verbs=get;update;patch

func (r *VirtualNodeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	node := &corev1.Node{}
	if err := r.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof(" -------------- Node not found --------------")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		} else {
			klog.Errorln(err, " -------------- Unable to get the node --------------")
			return ctrl.Result{}, err
		}
	}

	nm := &namespaceresourcesv1.NamespaceMap{}
	if err := r.namespaceMapLifecycle(nm, node); err != nil {
		klog.Errorln(err, " -------------- Unable to get NamespaceMap --------------")
		return ctrl.Result{}, err
	}

	if node.GetDeletionTimestamp().IsZero() {
		if !containsString(node.GetFinalizers(), virtualNodeFinalizer) {
			node.SetFinalizers(append(node.GetFinalizers(), virtualNodeFinalizer))
			// TODO: also with patch i have conflict, others controllores update this node (is possible to avoid it?)
			if err := r.Patch(context.TODO(), node, client.Merge); err != nil {
				klog.Errorln(err, " -------------- Unable to add finalizer --------------")
				return ctrl.Result{}, err
			}
		}
	} else {
		if containsString(node.GetFinalizers(), virtualNodeFinalizer) {
			// TODO: decide what to do here
			if err := r.namespaceMapDeletionLogic(nm); err != nil {
				return ctrl.Result{}, err
			}

			klog.Infof("Someone try to delete virtual node, ok delete!!")

			node.SetFinalizers(removeString(node.GetFinalizers(), virtualNodeFinalizer))
			// TODO: Update to patch.apply()
			if err := r.Update(context.Background(), node); err != nil {
				klog.Errorln(err, " -------------- Unable to remove finalizer --------------")
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted, now is not useful but if the logic changes may be useful
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// Events not filtered:
// 1 -- add of a new virtual-node with right label "type"
// 2 -- update deletionTimestamp on NamespaceMap or on virtual-node, due to deletion request
func createOrDeleteNamespaceMap() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {

			// this avoid update events for nodes which are not virtual
			if e.MetaNew.GetNamespace() != mapNamespaceName {
				if value, ok := (e.MetaNew.GetLabels())["type"]; !ok || value != "virtual-node" {
					return false
				}
			}

			return !(e.MetaNew.GetDeletionTimestamp().IsZero())
		},
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object.GetObjectKind().GroupVersionKind().Kind == "NamespaceMap" {
				return false
			}

			value, ok := (e.Meta.GetLabels())["type"]
			return ok && value == "virtual-node"
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		// Todo: to evaluate if remove also this
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func (r *VirtualNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Owns(&namespaceresourcesv1.NamespaceMap{}).
		WithEventFilter(createOrDeleteNamespaceMap()).
		WithOptions(controller.Options{MaxConcurrentReconciles: 0}). // for the moment no concurrency
		Complete(r)
}
