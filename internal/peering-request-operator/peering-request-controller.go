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
	"errors"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterID"
	"github.com/liqotech/liqo/pkg/crdClient"
	object_references "github.com/liqotech/liqo/pkg/object-references"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// PeeringRequestReconciler reconciles a PeeringRequest object
type PeeringRequestReconciler struct {
	Scheme *runtime.Scheme

	crdClient                 *crdClient.CRDClient
	Namespace                 string
	clusterId                 clusterID.ClusterID
	broadcasterImage          string
	broadcasterServiceAccount string
	vkServiceAccount          string
	retryTimeout              time.Duration

	// testing
	ForeignConfig *rest.Config
}

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=peeringrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=peeringrequests/status,verbs=get;update;patch

func (r *PeeringRequestReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()

	klog.Info("Reconciling PeeringRequest " + req.Name)

	tmp, err := r.crdClient.Resource("peeringrequests").Get(req.Name, metav1.GetOptions{})
	if err != nil {
		// TODO: has been removed
		klog.Info(err, "Destroy peering")
		return ctrl.Result{RequeueAfter: r.retryTimeout}, nil
	}
	pr, ok := tmp.(*discoveryv1alpha1.PeeringRequest)
	if !ok {
		klog.Error("loaded object is not a PeeringRequest")
		return ctrl.Result{RequeueAfter: r.retryTimeout}, errors.New("loaded object is not a PeeringRequest")
	}
	if pr.Spec.KubeConfigRef == nil {
		return ctrl.Result{}, nil
	}

	err = r.UpdateForeignCluster(pr)
	if err != nil {
		klog.Error(err, err.Error())
		return ctrl.Result{RequeueAfter: r.retryTimeout}, err
	}

	exists := pr.Status.BroadcasterRef != nil
	if exists {
		// check if it really exists
		exists, err = r.BroadcasterExists(pr)
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{RequeueAfter: r.retryTimeout}, err
		}
	}
	if !exists {
		klog.Info("Deploy Broadcaster")
		deploy := GetBroadcasterDeployment(pr, r.broadcasterServiceAccount, r.vkServiceAccount, r.Namespace, r.broadcasterImage, r.clusterId.GetClusterID())
		deploy, err = r.crdClient.Client().AppsV1().Deployments(r.Namespace).Create(context.TODO(), deploy, metav1.CreateOptions{})
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{RequeueAfter: r.retryTimeout}, err
		}
		pr.Status.BroadcasterRef = &object_references.DeploymentReference{
			Namespace: deploy.Namespace,
			Name:      deploy.Name,
		}
	}

	_, err = r.crdClient.Resource("peeringrequests").Update(pr.Name, pr, metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		return ctrl.Result{RequeueAfter: r.retryTimeout}, err
	}
	klog.Info("PeeringRequest " + pr.Name + " successfully reconciled")
	return ctrl.Result{RequeueAfter: r.retryTimeout}, nil
}

func (r *PeeringRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.PeeringRequest{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
