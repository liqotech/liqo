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

package controllers

import (
	namespaceresourcesv1 "github.com/liqotech/liqo/apis/virtualKubelet/v1"
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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
	clusterId             = "cluster-id"
)

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


func (r *NamespaceReconciler) removeRemoteNamespaces(localName string, nms map[string]*namespaceresourcesv1.NamespaceMap) error {

	for _, nm := range nms {

		if remoteName, ok := nm.Status.NattingTable[localName]; ok {
			//payload := []patchStringValue{{
			//	Op:    "remove",
			//	Path:  "/spec/deNattingTable",
			//	Value: deNat,
			//}}
			//payloadBytes, _ := json.Marshal(payload)
			//if err := r.Patch(context.Background(), &e,client.RawPatch(types.JSONPatchType, payloadBytes)); err != nil {
			//	klog.Error(err, " --> Unable to Update namespaceNattingTables")
			//}

			//payload = []patchStringValue{{
			//	Op:    "remove",
			//	Path:  "/spec/nattingTable",
			//	Value: name,
			//}}
			//payloadBytes, _ = json.Marshal(payload)
			//if err := r.Patch(context.Background(), &e,client.RawPatch(types.JSONPatchType, payloadBytes)); err != nil {
			//	klog.Error(err, " --> Unable to Update namespaceNattingTables")
			//}

			//entry := make(map[string]string)
			//entry["pippo"] = deNat
			//payload := []patchMapValue{{
			//	Op:    "add",
			//	Path:  "/spec/deNattingTable",
			//	Value: entry,
			//}}
			//payloadBytes, _ := json.Marshal(payload)
			//if err := r.Patch(context.Background(), &e,client.RawPatch(types.JSONPatchType, payloadBytes)); err != nil {
			//	klog.Error(err, " --> Unable to Update namespaceNattingTables")
			//}

			delete(nm.Status.NattingTable, localName)
			delete(nm.Status.DeNattingTable, remoteName)
			// TODO: Update to Patch.apply()
			if err := r.Update(context.TODO(), nm); err != nil {
				klog.Errorln(err, " -------------- Unable to update NamespaceMap --------------")
				return err
			}
			klog.Infof("Entries deleted correctly")

			// ----------------- REMOVE REMOTE NAMESPACE ---------------------
			// ---------------------------------------------------- still todo
			// ---------------------------------------------------------------

		}
	}

	return nil
}

func (r *NamespaceReconciler) createRemoteNamespace(n *corev1.Namespace, remoteName string, nm *namespaceresourcesv1.NamespaceMap) error {

	localName := n.GetName()

	if nm.Status.NattingTable == nil {
		nm.Status.NattingTable = map[string]string{}
	}

	if nm.Status.DeNattingTable == nil {
		nm.Status.DeNattingTable = map[string]string{}
	}

	if oldValue, ok := nm.Status.NattingTable[localName]; ok {
		// case in which mapping is already present but with different name
		if oldValue != remoteName {
			n.Labels[mappingLabel] = oldValue // TODO: this triggers again reconcile, to consider how to avoid, also if there is no problem to trigger it again
			// TODO: Update to Patch
			if err := r.Update(context.TODO(), n); err != nil {
				klog.Errorln(err, " -------------- Unable to update mapping label --------------")
				return err
			}
		}
	} else {
		// case in which there is no mapping
		nm.Status.NattingTable[localName] = remoteName
		nm.Status.DeNattingTable[remoteName] = localName

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
		// if there are no namespaceMaps in the cluster, List doesn't trigger an error
		klog.Errorln(err, " -------------- Unable to List NamespaceMaps --------------")
		return ctrl.Result{}, err
	}

	// TODO: in case of no namespaceMap, do nothing
	if len(namespaceMaps.Items) == 0 {
		klog.Infof("No namespaceMaps at the moment")
		return ctrl.Result{}, nil
	}

	erase := make(map[string]*namespaceresourcesv1.NamespaceMap)
	for i, namespaceMap := range namespaceMaps.Items {
		erase[namespaceMap.Spec.RemoteClusterId] = &(namespaceMaps.Items[i])
	}

	d := len(erase)

	if namespace.GetDeletionTimestamp().IsZero() {
		if !containsString(namespace.GetFinalizers(), namespaceFinalizer) {
			namespace.SetFinalizers(append(namespace.GetFinalizers(), namespaceFinalizer))
			if err := r.Patch(context.TODO(), namespace, client.Merge); err != nil {
				klog.Errorln(err, " -------------- Unable to add finalizer --------------")
				return ctrl.Result{}, err
			}
		}
	} else {
		if containsString(namespace.GetFinalizers(), namespaceFinalizer) {

			if err := r.removeRemoteNamespaces(namespace.GetName(), erase); err != nil {
				return ctrl.Result{}, err
			}

			klog.Infof("Someone try to delete namespace, ok delete!!")

			namespace.SetFinalizers(removeString(namespace.GetFinalizers(), namespaceFinalizer))
			// TODO: Update to patch.apply()
			if err := r.Update(context.Background(), namespace); err != nil {
				klog.Errorln(err, " -------------- Unable to remove finalizer --------------")
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		klog.Infof("Success!!")
		return ctrl.Result{}, nil
	}

	labels := namespace.Labels

	// 1. If mapping.liqo.io label is not present there are no remote namespaces associated with this one, erase is full
	if remoteNamespaceName, ok := labels[mappingLabel]; ok {

		// 2.a If offloading.liqo.io is present there are remote namespaces on all virual nodes
		if _, ok = labels[offloadingLabel]; ok {
			klog.Infof("Create on all clusters")
			if err := r.createRemoteNamespaces(namespace, remoteNamespaceName, namespaceMaps); err != nil {
				return ctrl.Result{}, err
			}

			for k := range erase {
				delete(erase, k)
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

			// TODO: in case of no virtualNode, do nothing
			if len(nodes.Items) == 0 {
				klog.Infof("No VirtualNode at the moment")
				return ctrl.Result{}, nil
			}

			// TODO: "found" variable shouldn't be useful anymore, virtual node must have necessary labels at creation (or not?)
			for _, node := range nodes.Items {
				i := 0
				found := false
				dim := len(node.Labels)
				id := node.Annotations[clusterId]

				for key := range node.Labels {
					i++
					if len(key) > 18 && key[0:19] == offloadingPrefixLabel {
						if _, ok = labels[key]; !ok {
							klog.Infof("Not create remote namespace on: " + id)
							break
						} else {
							found = true // found will be no more necessary with only virtual nodes
						}
					}

					if i == dim && found {
						klog.Infof("Create namespace for remote cluster: " + id)
						if err := r.createRemoteNamespace(namespace, remoteNamespaceName, erase[id]); err != nil {
							return ctrl.Result{}, err
						}
						delete(erase, id)
					}

				}
			}
		}

	}

	if len(erase) > 0 {
		klog.Infof("Delete all unnecessary mapping in NNT")
		if err := r.removeRemoteNamespaces(namespace.GetName(), erase); err != nil {
			return ctrl.Result{}, err
		}
	}

	// TODO: is useful ?
	if len(erase) == d {
		// finalizer no more useful on this namespace
		if containsString(namespace.GetFinalizers(), namespaceFinalizer) {
			namespace.SetFinalizers(removeString(namespace.GetFinalizers(), namespaceFinalizer))
			// TODO: Update to patch.apply()
			if err := r.Update(context.Background(), namespace); err != nil {
				klog.Errorln(err, " -------------- Unable to remove finalizer --------------")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// This function is useful only if we decide to accept the renaming of namespaces in "mapping.liqo.io" label.
// If we don't want to change the name of namespaces, in function reconcile when we check that the name in the NNT is
// different from the new name, with client we change the value of the new inserted label
func mappingLabelUpdate(old map[string]string, new map[string]string) bool {
	ret := false
	if val1, ok := old[mappingLabel]; ok {
		ret = val1 != new[mappingLabel]
	}
	return ret
}

// only a simple check in order to avoid useless overhead
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

			// if namespace doesn't have my finalizer, i don't care of it
			if !(e.MetaNew.GetDeletionTimestamp().IsZero()) && containsString(e.MetaNew.GetFinalizers(), namespaceFinalizer) {
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
		GenericFunc: func(e event.GenericEvent) bool {
			return true
		},
	}
}

func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		WithEventFilter(manageLabelPredicate()).
		Complete(r)
}
