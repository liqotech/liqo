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

package peering_request_operator

import (
	"context"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/pkg/clusterID"
	v1 "github.com/liqoTech/liqo/pkg/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
)

// PeeringRequestReconciler reconciles a PeeringRequest object
type PeeringRequestReconciler struct {
	Scheme *runtime.Scheme

	client                    *kubernetes.Clientset
	discoveryClient           *v1.DiscoveryV1Client
	Namespace                 string
	clusterId                 *clusterID.ClusterID
	configMapName             string
	broadcasterImage          string
	broadcasterServiceAccount string
}

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=peeringrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=peeringrequests/status,verbs=get;update;patch

func (r *PeeringRequestReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()

	pr, err := r.discoveryClient.PeeringRequests().Get(req.Name, metav1.GetOptions{})
	if err != nil {
		// TODO: has been removed
		klog.Info("Destroy peering")
		return ctrl.Result{}, nil
	}
	if pr.Spec.KubeConfigRef == nil {
		return ctrl.Result{}, nil
	}

	exists, err := r.BroadcasterExists(pr)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !exists {
		klog.Info("Deploy Broadcaster")
		cm, err := r.client.CoreV1().ConfigMaps(r.Namespace).Get(r.configMapName, metav1.GetOptions{})
		if err != nil {
			return ctrl.Result{}, err
		}
		deploy := GetBroadcasterDeployment(pr, r.broadcasterServiceAccount, r.Namespace, r.broadcasterImage, r.clusterId.GetClusterID(), cm.Data["gatewayIP"], cm.Data["gatewayPrivateIP"])
		_, err = r.client.AppsV1().Deployments(r.Namespace).Create(&deploy)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *PeeringRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1.PeeringRequest{}).
		Complete(r)
}
