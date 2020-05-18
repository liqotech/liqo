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

package foreign_cluster_operator

import (
	"context"
	"github.com/netgroup-polito/dronev2/internal/discovery"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"log"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1 "github.com/netgroup-polito/dronev2/api/discovery/v1"
)

// ForeignClusterReconciler reconciles a ForeignCluster object
type ForeignClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=discovery.drone.com,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.drone.com,resources=foreignclusters/status,verbs=get;update;patch

func (r *ForeignClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("foreigncluster", req.NamespacedName)

	discoveryClient, _ := discovery.NewDiscoveryClient()
	fc, err := discoveryClient.ForeignClusters(req.Namespace).Get(req.Name, metav1.GetOptions{})
	if err != nil {
		// TODO: has been removed
		return ctrl.Result{}, err
	}

	clientConfig, _ := fc.GetConfig()

	// TODO: create a federation request if it not exists
	k8sClient, _ := kubernetes.NewForConfig(clientConfig)
	pods, err := k8sClient.CoreV1().Pods(apiv1.NamespaceDefault).List(metav1.ListOptions{})
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, pod := range pods.Items {
		log.Println(pod.Name)
	}

	return ctrl.Result{}, nil
}

func (r *ForeignClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1.ForeignCluster{}).
		Complete(r)
}
