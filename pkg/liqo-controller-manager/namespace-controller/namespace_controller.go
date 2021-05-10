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

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	mappingLabel                 = "mapping.liqo.io"
	offloadingLabel              = "offloading.liqo.io"
	offloadingPrefixLabel        = "offloading.liqo.io/"
	namespaceControllerFinalizer = "namespace-controller.liqo.io/finalizer"
)

func (r *NamespaceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {

	namespace := &corev1.Namespace{}
	if err := r.Get(context.TODO(), req.NamespacedName, namespace); err != nil {
		klog.Errorf("%s --> Unable to get namespace '%s'", err, req.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	namespaceMaps := &mapsv1alpha1.NamespaceMapList{}
	if err := r.List(context.TODO(), namespaceMaps); err != nil {
		klog.Error(err, " --> Unable to List NamespaceMaps")
		return ctrl.Result{}, err
	}

	if len(namespaceMaps.Items) == 0 {
		klog.Info(" No namespaceMaps at the moment in the cluster")
		return ctrl.Result{}, nil
	}

	removeMappings := make(map[string]mapsv1alpha1.NamespaceMap)
	for _, namespaceMap := range namespaceMaps.Items {
		removeMappings[namespaceMap.GetLabels()[liqoconst.VirtualNodeClusterId]] = namespaceMap
	}

	if !namespace.GetDeletionTimestamp().IsZero() {

		klog.Infof("The namespace '%s' is requested to be deleted", namespace.GetName())
		if err := r.removeDesiredMappings(namespace.GetName(), removeMappings); err != nil {
			return ctrl.Result{}, err
		}
		ctrlutils.RemoveFinalizer(namespace, namespaceControllerFinalizer)

		if err := r.Update(context.TODO(), namespace); err != nil {
			klog.Errorf("%s --> Unable to remove finalizer from namespace '%s'", err, namespace.GetName())
			return ctrl.Result{}, err
		}
		klog.Infof("Finalizer is correctly removed from namespace'%s'", namespace.GetName())

		return ctrl.Result{}, nil
	}

	if !ctrlutils.ContainsFinalizer(namespace, namespaceControllerFinalizer) {
		ctrlutils.AddFinalizer(namespace, namespaceControllerFinalizer)
		if err := r.Patch(context.TODO(), namespace, client.Merge); err != nil {
			klog.Errorf(" %s --> Unable to add finalizer on namespace '%s'", err, namespace.GetName())
			return ctrl.Result{}, err
		}
	}

	// 1. If mapping.liqo.io label is not present there are no remote namespaces associated with this namespace, removeMappings is full
	if remoteNamespaceName, ok := namespace.GetLabels()[mappingLabel]; ok {

		// 2.a If offloading.liqo.io is present there are remote namespaces on all virtual nodes
		if _, ok = namespace.GetLabels()[offloadingLabel]; ok {
			klog.Infof(" Offload namespace '%s' on all remote clusters", namespace.GetName())
			if err := r.addDesiredMappings(namespace, remoteNamespaceName, removeMappings); err != nil {
				return ctrl.Result{}, err
			}

			for k := range removeMappings {
				delete(removeMappings, k)
			}

		} else {

			// 2.b Iterate on all virtual nodes' labels, if the namespace has all the requested labels, is necessary to
			// offload it onto remote cluster associated with the virtual node
			nodes := &corev1.NodeList{}
			if err := r.List(context.TODO(), nodes, client.MatchingLabels{liqoconst.TypeLabel: liqoconst.TypeNode}); err != nil {
				klog.Error(err, " --> Unable to List all virtual nodes")
				return ctrl.Result{}, err
			}

			if len(nodes.Items) == 0 {
				klog.Info(" No VirtualNode at the moment")
				return ctrl.Result{}, nil
			}

			for _, node := range nodes.Items {
				if checkOffloadingLabels(namespace, &node) {
					if err := r.addDesiredMapping(namespace, remoteNamespaceName, removeMappings[node.Annotations[liqoconst.VirtualNodeClusterId]]); err != nil {
						return ctrl.Result{}, err
					}
					delete(removeMappings, node.Annotations[liqoconst.VirtualNodeClusterId])
					klog.Infof(" Offload namespace '%s' on remote cluster: %s", namespace.GetName(), node.Annotations[liqoconst.VirtualNodeClusterId])
				}

			}
		}

	}

	if len(removeMappings) > 0 {
		klog.Info(" Delete all unnecessary entries in NamespaceMaps")
		if err := r.removeDesiredMappings(namespace.GetName(), removeMappings); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		WithEventFilter(manageLabelPredicate()).
		Complete(r)
}
