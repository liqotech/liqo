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

package advertisement_operator

import (
	"context"
	"github.com/netgroup-polito/dronev2/pkg/advertisement-operator"

	"github.com/go-logr/logr"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	protocolv1beta1 "github.com/netgroup-polito/dronev2/api/v1beta1"
)

// AdvertisementReconciler reconciles a Advertisement object
type AdvertisementReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=protocol.drone.com,resources=advertisements,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=protocol.drone.com,resources=advertisements/status,verbs=get;update;patch

func (r *AdvertisementReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("advertisement-controller", req.NamespacedName)

	// get advertisement
	var adv protocolv1beta1.Advertisement
	if err := r.Get(ctx, req.NamespacedName, &adv); err != nil {
		log.Error(err, "unable to fetch Advertisement")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// filter advertisements and create a virtual-kubelet only for the good ones
	adv = checkAdvertisement(adv)
	if adv.Status.Phase != "ACCEPTED" {
		return ctrl.Result{}, errors.NewBadRequest("advertisement ignored")
	}

	// create configuration for virtual-kubelet with data from adv
	vkConfig := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vk-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"vkubelet-cfg.json": `
		{
		 "virtual-kubelet": {
		   "remoteKubeconfig" : "/app/kubeconfig/remote",
		   "namespace": "drone-v2",
		   "cpu": "` + adv.Spec.Availability.Cpu().String() + `",
		   "memory": "` + adv.Spec.Availability.Memory().String() + `",
		   "pods": "` + adv.Spec.Availability.Pods().String() + `",
		   "remoteNewPodCidr": "172.48.0.0/16"
		 }
		}`},
	}
	err := advertisement_operator.CreateOrUpdate(r.Client, ctx, log, vkConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	// create resources necessary to run virtual-kubelet deployment
	err = advertisement_operator.CreateFromYaml(r.Client, ctx, log, "./data/vk_sa.yaml", "ServiceAccount")
	if err != nil {
		return ctrl.Result{}, err
	}
	err = advertisement_operator.CreateFromYaml(r.Client, ctx, log, "./data/vk_crb.yaml", "ClusterRoleBinding")
	if err != nil {
		return ctrl.Result{}, err
	}
	err = advertisement_operator.CreateFromYaml(r.Client, ctx, log, "./data/vk_deploy.yaml", "Deployment")
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *AdvertisementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&protocolv1beta1.Advertisement{}).
		Complete(r)
}

// check if the advertisement is interesting and set its status accordingly
func checkAdvertisement(adv protocolv1beta1.Advertisement) protocolv1beta1.Advertisement {
	//TODO: implement logic
	adv.Status.Phase = "ACCEPTED"
	return adv
}
