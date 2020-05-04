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
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/record"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	protocolv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	pkg "github.com/netgroup-polito/dronev2/pkg/advertisement-operator"
)

// AdvertisementReconciler reconciles a Advertisement object
type AdvertisementReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	EventsRecorder  record.EventRecorder
}

// +kubebuilder:rbac:groups=protocol.drone.com,resources=advertisements,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=protocol.drone.com,resources=advertisements/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events/status,verbs=get

func (r *AdvertisementReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("advertisement-controller", req.NamespacedName)

	// get advertisement
	var adv protocolv1.Advertisement
	if err := r.Get(ctx, req.NamespacedName, &adv); err != nil {
		// reconcile was triggered by a delete request
		log.Info("Advertisement " + req.Name + " deleted")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	namespace := "drone-" + adv.Spec.ClusterId

	// The metadata.generation value is incremented for all changes, except for changes to .metadata or .status
	// if metadata.generation is not incremented there's no need to reconcile
	if adv.Status.ObservedGeneration == adv.ObjectMeta.Generation {
		return ctrl.Result{}, nil
	}

	// get nodes of the local cluster
	nodes, err := GetNodes(r.Client, ctx, log)
	if err != nil{
		return ctrl.Result{}, err
	}
	// filter advertisements and create a virtual-kubelet only for the good ones
	checkAdvertisement(r, ctx, log, &adv, nodes)

	if adv.Status.AdvertisementStatus != "ACCEPTED" {
		return ctrl.Result{}, errors.NewBadRequest("advertisement ignored")
	}

	// create configuration for virtual-kubelet with data from adv
	vkConfig := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vk-config-" + adv.Spec.ClusterId,
			Namespace: "default",
			OwnerReferences: pkg.GetOwnerReference(adv),
		},
		Data: map[string]string{
			"vkubelet-cfg.json": `
		{
		 "vk-` + adv.Spec.ClusterId + `": {
		   "remoteKubeconfig": "/app/kubeconfig/remote",
		   "namespace": "` + namespace +`",
		   "cpu": "` + adv.Spec.Availability.Cpu().String() + `",
		   "memory": "` + adv.Spec.Availability.Memory().String() + `",
		   "pods": "` + adv.Spec.Availability.Pods().String() + `",
		   "remoteNewPodCidr": "` + adv.Spec.Network.PodCIDR + `"
		 }
		}`},
	}
	err = pkg.CreateOrUpdate(r.Client, ctx, log, vkConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	deploy := pkg.CreateVkDeployment(adv)
	err = pkg.CreateOrUpdate(r.Client, ctx, log, deploy)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("launching virtual-kubelet for cluster " + adv.Spec.ClusterId)

	// start the reflector only if this is the first time we receive this advertisement
	if adv.Generation == 1 {
		StartReflector(namespace, adv)
	}
	return ctrl.Result{}, nil
}

func (r *AdvertisementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&protocolv1.Advertisement{}).
		Complete(r)
}

func GetNodes(c client.Client , ctx context.Context, log logr.Logger) ([]v1.Node, error) {
	var nodes v1.NodeList

	selector, err := labels.Parse("type != virtual-node")
	if err = c.List(ctx, &nodes, client.MatchingLabelsSelector{Selector:selector}) ; err != nil {
		log.Error(err, "Unable to list nodes")
		return nil, err
	}
	return nodes.Items, nil
}

// check if the advertisement is interesting and set its status accordingly
func checkAdvertisement(r *AdvertisementReconciler, ctx context.Context, log logr.Logger,
	adv *protocolv1.Advertisement, nodes []v1.Node) {

	//TODO: implement logic
	adv.Status.AdvertisementStatus = "ACCEPTED"
	metav1.SetMetaDataAnnotation(&adv.ObjectMeta, "advertisementStatus", "accepted")

	recordEvent(r, log, "Advertisement " + adv.Name + " accepted", "Normal", "AdvertisementAccepted", adv)
	adv.Status.ForeignNetwork = protocolv1.NetworkInfo{
		PodCIDR:            GetPodCIDR(nodes),
	}
	adv.Status.ObservedGeneration = adv.ObjectMeta.Generation
	if err := r.Status().Update(ctx, adv); err != nil {
		log.Error(err, "unable to update Advertisement status")
	}
	return
}

func recordEvent(r *AdvertisementReconciler, log logr.Logger,
	msg string, eventType string, eventReason string,
	adv *protocolv1.Advertisement) {

	log.Info(msg)
	r.EventsRecorder.Event(adv, eventType, eventReason, msg)

	return
}