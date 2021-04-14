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

package namespace_controller

import (
	"context"
	namespaceresourcesv1 "github.com/liqotech/liqo/apis/virtualKubelet/v1"
	liqoctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/slice"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
)

type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	mappingLabel          = "mapping.liqo.io"
	offloadingLabel       = "offloading.liqo.io"
	offloadingPrefixLabel = "offloading.liqo.io/"
	namespaceFinalizer    = "namespace.liqo.io/finalizer"
)

func (r *NamespaceReconciler) removeRemoteNamespace(localName string, nm *namespaceresourcesv1.NamespaceMap) error {

	if remoteName, ok := nm.Status.NattingTable[localName]; ok {

		delete(nm.Status.NattingTable, localName)
		delete(nm.Status.DeNattingTable, remoteName)
		if err := r.Update(context.TODO(), nm); err != nil {
			klog.Errorln(err, " -------------- Unable to update NamespaceMap --------------")
			return err
		}
		klog.Infof("Entries deleted correctly")

		// ----------------- REMOVE REMOTE NAMESPACE ---------------------
		// ---------------------------------------------------- still todo
		// ---------------------------------------------------------------

	}

	return nil
}

func (r *NamespaceReconciler) removeRemoteNamespaces(localName string, nms map[string]*namespaceresourcesv1.NamespaceMap) error {

	for _, nm := range nms {
		if err := r.removeRemoteNamespace(localName, nm); err != nil {
			return err
		}
	}
	return nil
}

func (r *NamespaceReconciler) createRemoteNamespace(n *corev1.Namespace, remoteName string, nm *namespaceresourcesv1.NamespaceMap) error {

	if nm.Status.NattingTable == nil {
		nm.Status.NattingTable = map[string]string{}
	}

	if nm.Status.DeNattingTable == nil {
		nm.Status.DeNattingTable = map[string]string{}
	}

	if oldValue, ok := nm.Status.NattingTable[n.GetName()]; ok {
		if oldValue != remoteName {
			n.GetLabels()[mappingLabel] = oldValue
			if err := r.Update(context.TODO(), n); err != nil {
				klog.Errorln(err, " -------------- Unable to update mapping label --------------")
				return err
			}
		}
	} else {
		nm.Status.NattingTable[n.GetName()] = remoteName
		nm.Status.DeNattingTable[remoteName] = n.GetName()

		// ----------------- CREATE REMOTE NAMESPACE ---------------------
		// ---------------------------------------------------- still todo
		// ---------------------------------------------------------------

		if err := r.Patch(context.TODO(), nm, client.Merge); err != nil {
			klog.Errorln(err, " -------------- Unable to add entries in NamespaceMap --------------")
			return err
		}

	}
	return nil
}

func (r *NamespaceReconciler) createRemoteNamespaces(n *corev1.Namespace, remoteName string, nms *namespaceresourcesv1.NamespaceMapList) error {

	for _, nm := range nms.Items {
		if err := r.createRemoteNamespace(n, remoteName, &nm); err != nil {
			return err
		}
	}
	return nil
}

func (r *NamespaceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, req.NamespacedName, namespace); err != nil {
		if errors.IsNotFound(err) {
			klog.Errorln(err, " -------------- Namespace not found --------------")
			return ctrl.Result{}, nil
		} else {
			klog.Errorln(err, " -------------- Unable to get namespace --------------")
			return ctrl.Result{}, err
		}
	}

	namespaceMaps := &namespaceresourcesv1.NamespaceMapList{}
	if err := r.List(context.Background(), namespaceMaps); err != nil {
		klog.Errorln(err, " -------------- Unable to List NamespaceMaps --------------")
		return ctrl.Result{}, err
	}

	if len(namespaceMaps.Items) == 0 {
		klog.Infof("No namespaceMaps at the moment")
		return ctrl.Result{}, nil
	}

	removeMappings := make(map[string]*namespaceresourcesv1.NamespaceMap)
	for i, namespaceMap := range namespaceMaps.Items {
		removeMappings[namespaceMap.Spec.RemoteClusterId] = &(namespaceMaps.Items[i])
	}

	if namespace.GetDeletionTimestamp().IsZero() {
		if !slice.ContainsString(namespace.GetFinalizers(), namespaceFinalizer, nil) {
			namespace.SetFinalizers(append(namespace.GetFinalizers(), namespaceFinalizer))
			if err := r.Patch(context.TODO(), namespace, client.Merge); err != nil {
				klog.Errorln(err, " -------------- Unable to add finalizer --------------")
				return ctrl.Result{}, err
			}
		}
	} else {
		if slice.ContainsString(namespace.GetFinalizers(), namespaceFinalizer, nil) {

			if err := r.removeRemoteNamespaces(namespace.GetName(), removeMappings); err != nil {
				return ctrl.Result{}, err
			}

			klog.Infof("Someone try to delete namespace, ok delete!!")

			namespace.SetFinalizers(slice.RemoveString(namespace.GetFinalizers(), namespaceFinalizer, nil))
			if err := r.Update(context.Background(), namespace); err != nil {
				klog.Errorln(err, " -------------- Unable to remove finalizer --------------")
				return ctrl.Result{}, err
			}
		}

		klog.Infof("Namespace deleted!!")
		return ctrl.Result{}, nil
	}

	// 1. If mapping.liqo.io label is not present there are no remote namespaces associated with this one, removeMappings is full
	if remoteNamespaceName, ok := namespace.GetLabels()[mappingLabel]; ok {

		// 2.a If offloading.liqo.io is present there are remote namespaces on all virual nodes
		if _, ok = namespace.GetLabels()[offloadingLabel]; ok {
			klog.Infof("Create on all clusters")
			if err := r.createRemoteNamespaces(namespace, remoteNamespaceName, namespaceMaps); err != nil {
				return ctrl.Result{}, err
			}

			for k := range removeMappings {
				delete(removeMappings, k)
			}

		} else {

			klog.Infof("Watch for virtual-nodes labels")

			// 2.b Iterate on all nodes labels (next step only virtual nodes), if the namespace has all the requested
			// labels, is necessary to create the remote namespace on the remote cluster associated with the virtual
			// node

			nodes := &corev1.NodeList{}
			if err := r.List(context.Background(), nodes, client.MatchingLabels{"type": "virtual-node"}); err != nil {
				klog.Errorln(err, " -------------- Unable to List all virtual nodes --------------")
				return ctrl.Result{}, err
			}

			if len(nodes.Items) == 0 {
				klog.Infof("No VirtualNode at the moment")
				return ctrl.Result{}, nil
			}

			for _, node := range nodes.Items {
				i := 0
				dim := len(node.Labels)
				id := node.Annotations[liqoctrl.VirtualNodeClusterId]
				for key := range node.Labels {
					i++
					if strings.HasPrefix(key, offloadingPrefixLabel) {
						if _, ok = namespace.GetLabels()[key]; !ok {
							klog.Infof("Not create remote namespace on: " + id)
							break
						}
					}

					if i == dim {
						klog.Infof("Create namespace for remote cluster: " + id)
						if err := r.createRemoteNamespace(namespace, remoteNamespaceName, removeMappings[id]); err != nil {
							return ctrl.Result{}, err
						}
						delete(removeMappings, id)
					}

				}
			}
		}

	}

	if len(removeMappings) > 0 {
		klog.Infof("Delete all unnecessary mapping in NNT")
		if err := r.removeRemoteNamespaces(namespace.GetName(), removeMappings); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func mappingLabelUpdate(oldLabels map[string]string, newLabels map[string]string) bool {
	ret := false
	if val1, ok := oldLabels[mappingLabel]; ok {
		ret = val1 != newLabels[mappingLabel]
	}
	return ret
}

func mappingLabelPresence(labels map[string]string) bool {
	_, ok := labels[mappingLabel]
	return ok
}

// Events not filtered:
// 1 -- add/delete labels, and mappingLabel is present before or after the update
// 2 -- update the value of mappingLabel label only
// 3 -- add namespace with at least mappingLabel
// 4 -- deletion timestamp is updated on a relevant namespace
func manageLabelPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {

			if !(e.MetaNew.GetDeletionTimestamp().IsZero()) && slice.ContainsString(e.MetaNew.GetFinalizers(), namespaceFinalizer, nil) {
				return true
			}

			oldLabels := e.MetaOld.GetLabels()
			newLabels := e.MetaNew.GetLabels()
			return ((len(oldLabels) != len(newLabels)) && (mappingLabelPresence(oldLabels) || mappingLabelPresence(newLabels))) || mappingLabelUpdate(oldLabels, newLabels)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return mappingLabelPresence(e.Meta.GetLabels())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
}

func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		WithEventFilter(manageLabelPredicate()).
		Complete(r)
}
