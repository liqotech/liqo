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

package namespaceMap_controller

import (
	"context"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	const_ctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type NamespaceMapReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Mapper        meta.RESTMapper
	RemoteClients map[string]client.Client
}

const (
	clusterIdForeign = "discovery.liqo.io/cluster-id"
)

func (r *NamespaceMapReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {

	namespaceMap := &mapsv1alpha1.NamespaceMap{}
	if err := r.Get(context.TODO(), req.NamespacedName, namespaceMap); err != nil {
		klog.Errorf("%s --> Unable to get NamespaceMap '%s'", err, req.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !ctrlutils.ContainsFinalizer(namespaceMap, const_ctrl.NamespaceMapControllerFinalizer) {
		ctrlutils.AddFinalizer(namespaceMap, const_ctrl.NamespaceMapControllerFinalizer)
		if err := r.Patch(context.TODO(), namespaceMap, client.Merge); err != nil {
			klog.Errorf("%s --> Unable to add '%s' to the NamespaceMap '%s'", err, const_ctrl.NamespaceMapControllerFinalizer, namespaceMap.GetName())
			return ctrl.Result{}, err
		}
		klog.Infof("Finalizer correctly added on NamespaceMap '%s'", namespaceMap.GetName())
	}

	if err := r.manageRemoteNamespaces(namespaceMap); err != nil {
		return ctrl.Result{}, err
	}

	if len(namespaceMap.Status.CurrentMapping) == 0 {
		ctrlutils.RemoveFinalizer(namespaceMap, const_ctrl.NamespaceMapControllerFinalizer)
		if err := r.Update(context.TODO(), namespaceMap); err != nil {
			klog.Errorf("%s --> Unable to remove '%s' from NamespaceMap '%s'", err, const_ctrl.NamespaceMapControllerFinalizer, namespaceMap.GetName())
			return ctrl.Result{}, err
		}
		klog.Infof("Finalizer correctly removed from NamespaceMap '%s'", namespaceMap.GetName())
	}

	return ctrl.Result{}, nil
}

// The Controller is triggered only when the number of entries in DesiredMapping changes, so only when
// a namespace's request is added or removed
func manageDesiredMappings() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return len(e.ObjectOld.(*mapsv1alpha1.NamespaceMap).Spec.DesiredMapping) != len(e.ObjectNew.(*mapsv1alpha1.NamespaceMap).Spec.DesiredMapping)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
}

func (r *NamespaceMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mapsv1alpha1.NamespaceMap{}).
		WithEventFilter(manageDesiredMappings()).
		Complete(r)
}
