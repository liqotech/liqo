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
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	pkg "github.com/liqoTech/liqo/pkg/advertisement-operator"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AdvertisementReconciler reconciles a Advertisement object
type AdvertisementReconciler struct {
	client.Client
	Log              logr.Logger
	Scheme           *runtime.Scheme
	EventsRecorder   record.EventRecorder
	GatewayIP        string
	GatewayPrivateIP string
	KubeletNamespace string
	KindEnvironment  bool
	VKImage          string
	InitVKImage      string
	HomeClusterId    string
}

// +kubebuilder:rbac:groups=protocol.liqo.io,resources=advertisements,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=protocol.liqo.io,resources=advertisements/status,verbs=get;update;patch
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

	// filter advertisements and create a virtual-kubelet only for the good ones
	if adv.Status.AdvertisementStatus == "" {
		// get nodes of the local cluster
		nodes, err := GetNodes(r.Client, ctx, log)
		if err != nil {
			return ctrl.Result{}, err
		}
		checkAdvertisement(r, ctx, log, &adv, nodes)
		return ctrl.Result{}, nil
	}

	if adv.Status.AdvertisementStatus != "ACCEPTED" {
		return ctrl.Result{}, errors.NewBadRequest("advertisement ignored")
	}

	if !r.KindEnvironment && adv.Status.RemoteRemappedPodCIDR == "" {
		r.Log.Info("advertisement not complete, remoteRemappedPodCIRD not set yet")
		return ctrl.Result{}, nil
	}

	if !adv.Status.VkCreated {
		err := createVirtualKubelet(r, ctx, log, &adv)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *AdvertisementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&protocolv1.Advertisement{}).
		Complete(r)
}

func GetNodes(c client.Client, ctx context.Context, log logr.Logger) ([]v1.Node, error) {
	var nodes v1.NodeList

	selector, err := labels.Parse("type != virtual-node")
	if err != nil {
		return nil, err
	}
	if err = c.List(ctx, &nodes, client.MatchingLabelsSelector{Selector: selector}); err != nil {
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

	recordEvent(r, log, "Advertisement "+adv.Name+" accepted", "Normal", "AdvertisementAccepted", adv)
	adv.Status.ForeignNetwork = protocolv1.NetworkInfo{
		PodCIDR:          GetPodCIDR(nodes),
		GatewayIP:        r.GatewayIP,
		GatewayPrivateIP: r.GatewayPrivateIP,
	}
	adv.Status.ObservedGeneration = adv.ObjectMeta.Generation
	if err := r.Status().Update(ctx, adv); err != nil {
		log.Error(err, "unable to update Advertisement status")
	}
}

func createVirtualKubelet(r *AdvertisementReconciler, ctx context.Context, log logr.Logger, adv *protocolv1.Advertisement) error {

	// Create the base resources
	vkSa := v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "vkubelet-" + adv.Spec.ClusterId,
			Namespace:       r.KubeletNamespace,
			OwnerReferences: pkg.GetOwnerReference(*adv),
		},
	}
	err := pkg.CreateOrUpdate(r.Client, ctx, log, vkSa)
	if err != nil {
		return err
	}
	vkCrb := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "vkubelet-" + adv.Spec.ClusterId,
			OwnerReferences: pkg.GetOwnerReference(*adv),
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", APIGroup: "", Name: "vkubelet-" + adv.Spec.ClusterId, Namespace: r.KubeletNamespace},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	}
	err = pkg.CreateOrUpdate(r.Client, ctx, log, vkCrb)
	if err != nil {
		return err
	}
	// Create the virtual Kubelet
	deploy := pkg.CreateVkDeployment(adv, vkSa.Name, r.KubeletNamespace, r.VKImage, r.InitVKImage, r.HomeClusterId)
	err = pkg.CreateOrUpdate(r.Client, ctx, log, deploy)
	if err != nil {
		return err
	}

	recordEvent(r, log, "launching virtual-kubelet for cluster "+adv.Spec.ClusterId, "Normal", "VkCreated", adv)
	adv.Status.VkCreated = true
	if err := r.Status().Update(ctx, adv); err != nil {
		log.Error(err, "unable to update Advertisement status")
	}
	return nil
}

func recordEvent(r *AdvertisementReconciler, log logr.Logger,
	msg string, eventType string, eventReason string,
	adv *protocolv1.Advertisement) {

	log.Info(msg)
	r.EventsRecorder.Event(adv, eventType, eventReason, msg)

}
