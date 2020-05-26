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

package federation_request_operator

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/netgroup-polito/dronev2/internal/discovery/clients"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1 "github.com/netgroup-polito/dronev2/api/discovery/v1"
)

// FederationRequestReconciler reconciles a FederationRequest object
type FederationRequestReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=discovery.drone.com,resources=federationrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.drone.com,resources=federationrequests/status,verbs=get;update;patch

func (r *FederationRequestReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("federationrequest", req.NamespacedName)

	discoveryClient, _ := clients.NewDiscoveryClient()
	fr, err := discoveryClient.FederationRequests().Get(req.Name, metav1.GetOptions{})
	if err != nil {
		// TODO: has been removed
		Log.Info("Destroy federation")
		return ctrl.Result{}, nil
	}

	// TODO: build federation
	Log.Info("Deploy Broadcaster")
	_, err = clients.NewK8sClient()
	if err != nil {
		return ctrl.Result{}, err
	}

	_ = GetBroadcasterDeployment(fr, "default", Namespace, "nginx:latest")

	// TODO: create/update deployment

	return ctrl.Result{}, nil
}

func (r *FederationRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1.FederationRequest{}).
		Complete(r)
}
