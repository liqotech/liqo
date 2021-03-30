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
	"context"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// TODO: put constant value instead of labels
// TODO: watch also the case of Namespace Creation, now is not managed, important !!!

type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	mappingLabel          = "mapping.liqo.io"
	offloadingLabel       = "offloading.liqo.io"
	offloadingPrefixLabel = "offloading.liqo.io/"
)

// TO EVALUATE: when is necessary to update namespace natting table (NNT), if the name exposed in mapping.liqo.io label,
// is different from the already present, client will change the label value to the old one. For users is not possible
// to change names of remote namespaces that have already been created with a certain name.

func (r *NamespaceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	// TODO: add namespace Finalizer
	var namespace v1.Namespace
	if err := r.Get(ctx, req.NamespacedName, &namespace); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("---------------------- I have to delete all entries from NNT, if present")
			return ctrl.Result{}, nil
		} else {
			klog.Error(err, " unable to fetch Namespace for some reasons")
			return ctrl.Result{}, err
		}
	}

	labels := namespace.Labels

	// TODO : var "erase" will become a map/vector with some references to all virtual nodes on which, remote namespace
	//        mustn't be present
	erase := true

	// 1. If mapping.liqo.io label is not present there are no remote namespaces associated with this one, erase is full
	if _, ok := labels[mappingLabel]; ok {

		// 2.a If offloading.liqo.io is present there are remote namespaces on all virual nodes
		if _, ok := labels[offloadingLabel]; ok {
			erase = false // TODO : erase must be empty, i will have remote namespaces on all virtual nodes
			klog.Infof("---------------------- I have to create remote namespaces on all virtual nodes, if they aren't already present")
		} else {

			klog.Infof("---------------------- Watch for virtual-nodes labels")

			// 2.b Iterate on all nodes labels (next step only virtual nodes), if the namespace has all the requested
			// labels, is necessary to create the remote namespace on the remote cluster associated with the virtual
			// node

			nodes := &v1.NodeList{}
			if err := r.List(context.Background(), nodes, client.MatchingLabels{"type": "virtual-node"}); err != nil {
				//if err := r.List(context.Background(), nodes); err != nil {
				klog.Error(err, "Unable to list virtual nodes")
				return ctrl.Result{}, err
			}

			for _, node := range nodes.Items {
				i := 0
				found := false
				dim := len(node.Labels)
				for key := range node.Labels {
					i++
					// I have to check only the offloading.liqo.io/ labels
					// virtual nodes will always have these offloading.liqo.io/ labels
					if len(key) > 18 && key[0:19] == offloadingPrefixLabel {
						if _, ok := labels[key]; !ok {
							break
						} else {
							found = true // found will be no more necessary with only virtual nodes
						}
					}

					if i == dim && found {
						klog.Infof("---------------------- Create namespace for that remote cluter")
						// TODO : remove from "erase" this virtual node
					}

				}
			}
		}

	}

	if erase {
		klog.Infof("---------------------- Delete all unnecessary mapping in NNT")
	}

	klog.Flush()
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
func manageLabelPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldLabels := e.MetaOld.GetLabels()
			newLabels := e.MetaNew.GetLabels()
			return ((len(oldLabels) != len(newLabels)) && (mappingLabelPresence(oldLabels) || mappingLabelPresence(newLabels))) || mappingLabelUpdate(oldLabels, newLabels)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false // i want to use only finalizer
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return mappingLabelPresence(e.Meta.GetLabels())
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false // to evaluate, now we don't consider this
		},
	}
}

func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Namespace{}).
		WithEventFilter(manageLabelPredicate()).
		Complete(r)
}
