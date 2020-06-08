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

package schedulingNodeOperator

import (
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SchedulingNodeReconciler reconciles a SchedulingNode object
type SchedulingNodeReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=scheduling.liqo.io,resources=schedulingnodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.liqo.io,resources=schedulingnodes/status,verbs=get;update;patch

func (r *SchedulingNodeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("schedulingnode", req.NamespacedName)

	// get nodes
	var no corev1.Node
	if err := r.Get(ctx, req.NamespacedName, &no); err != nil {

		if apierrors.IsNotFound(err) {
			// reconcile was triggered by a delete request
			log.Info("Node deleted")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		} else {
			return ctrl.Result{}, err
		}

	}

	return ctrl.Result{}, r.CreateOrUpdateFromNode(ctx, no)
}

// SetupWithManager registers the event handler for
// + node update,create,delete,patch
func (r *SchedulingNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r); err != nil {
		return err
	}

	return nil
}
