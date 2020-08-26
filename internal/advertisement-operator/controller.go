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
	goerrors "errors"
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	pkg "github.com/liqoTech/liqo/pkg/advertisement-operator"
	"github.com/liqoTech/liqo/pkg/crdClient"
	object_references "github.com/liqoTech/liqo/pkg/object-references"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"
)

const (
	AdvertisementAccepted = "Accepted"
	AdvertisementRefused  = "Refused"
	AdvertisementDeleting = "Deleting"
)

// AdvertisementReconciler reconciles a Advertisement object
type AdvertisementReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	EventsRecorder     record.EventRecorder
	KubeletNamespace   string
	KindEnvironment    bool
	VKImage            string
	InitVKImage        string
	HomeClusterId      string
	AcceptedAdvNum     int32
	ClusterConfig      policyv1.AdvertisementConfig
	AdvClient          *crdClient.CRDClient
	DiscoveryClient    *crdClient.CRDClient
	RetryTimeout       time.Duration
	garbaceCollector   sync.Once
	checkRemoteCluster map[string]*sync.Once
}

// +kubebuilder:rbac:groups=protocol.liqo.io,resources=advertisements,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=protocol.liqo.io,resources=advertisements/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events/status,verbs=get

func (r *AdvertisementReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	// start the advertisement garbage collector
	go r.garbaceCollector.Do(func() {
		r.cleanOldAdvertisements()
	})

	// initialize the checkRemoteCluster map
	if r.checkRemoteCluster == nil {
		r.checkRemoteCluster = make(map[string]*sync.Once)
	}

	// get advertisement
	var adv protocolv1.Advertisement
	if err := r.Get(ctx, req.NamespacedName, &adv); err != nil {
		if errors.IsNotFound(err) {
			// reconcile was triggered by a delete request
			klog.Info("Advertisement " + req.Name + " deleted")
			// TODO: decrease r.AcceptedAdvNum if the advertisement was ACCEPTED
			return ctrl.Result{RequeueAfter: r.RetryTimeout}, client.IgnoreNotFound(err)
		} else {
			// not managed error
			klog.Error(err)
			return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
		}
	}

	// we do that on Advertisement creation
	err, update := r.UpdateForeignCluster(&adv)
	if err != nil {
		klog.Warning(err)
		// this has not to return an error, Advertisement Operator will work fine
	}
	if update {
		err = r.Update(ctx, &adv, &client.UpdateOptions{})
		if err != nil {
			klog.Error(err)
			return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
		}
		// retry timeout not set because is an update, this will trigger Reconcile again
		return ctrl.Result{}, nil
	}

	// filter advertisements and create a virtual-kubelet only for the good ones
	if adv.Status.AdvertisementStatus == "" {
		r.CheckAdvertisement(&adv)
		r.UpdateAdvertisement(&adv)
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
	}

	if adv.Status.AdvertisementStatus != AdvertisementAccepted {
		klog.Info("Advertisement " + adv.Name + " refused")
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
	}

	if !r.KindEnvironment && adv.Status.RemoteRemappedPodCIDR == "" {
		klog.Info("advertisement not complete, remoteRemappedPodCIDR not set yet")
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
	}

	if !adv.Status.VkCreated {
		err := r.createVirtualKubelet(ctx, &adv)
		if err != nil {
			return ctrl.Result{}, err
		}
		klog.Info("Correct creation of virtual kubelet deployment for cluster " + adv.Spec.ClusterId)
		// start the keepalive check for the new cluster
		r.checkRemoteCluster[adv.Spec.ClusterId] = new(sync.Once)
		go r.checkRemoteCluster[adv.Spec.ClusterId].Do(func() {
			err := r.checkClusterStatus(adv)
			// if everything works well, the check is an infinite loop
			// therefore, if an error is returned, the foreign cluster is not reachable anymore
			if err != nil {
				// the foreign cluster is down: set adv status to trigger the unjoin
				klog.Error(err)
				adv.Status.AdvertisementStatus = AdvertisementDeleting
				if err := r.Status().Update(context.Background(), &adv); err != nil {
					klog.Error(err)
				}
			}
		})
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
	}

	return ctrl.Result{}, nil
}

func (r *AdvertisementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&protocolv1.Advertisement{}).
		Complete(r)
}

// set Advertisement reference in related ForeignCluster
func (r *AdvertisementReconciler) UpdateForeignCluster(adv *protocolv1.Advertisement) (error, bool) {
	tmp, err := r.DiscoveryClient.Resource("foreignclusters").List(metav1.ListOptions{
		LabelSelector: "cluster-id=" + adv.Spec.ClusterId,
	})
	if err != nil {
		klog.Error(err, err.Error())
		return err, false
	}
	fcList, ok := tmp.(*discoveryv1.ForeignClusterList)
	if !ok {
		err = goerrors.New("retrieved object is not a ForeignClusterList")
		klog.Error(err, err.Error())
		return err, false
	}
	if len(fcList.Items) == 0 {
		// object not found
		err = goerrors.New("ForeignCluster not found for cluster id " + adv.Spec.ClusterId)
		klog.Error(err, err.Error())
		return err, false
	}
	fc := fcList.Items[0]
	if !ok {
		err = goerrors.New("retrieved object is not a ForeignCluster")
		klog.Error(err, fc)
		return err, false
	}
	err = fc.SetAdvertisement(adv, r.DiscoveryClient)
	if err != nil {
		klog.Error(err, err.Error())
		return err, false
	}
	// check if FC is in Adv Owners
	contains := false
	for _, own := range adv.ObjectMeta.OwnerReferences {
		if own.UID == fc.UID {
			contains = true
			break
		}
	}
	if !contains {
		// add owner reference
		controller := true
		adv.OwnerReferences = append(adv.OwnerReferences, metav1.OwnerReference{
			APIVersion: "discovery.liqo.io/v1",
			Kind:       "ForeignCluster",
			Name:       fc.Name,
			UID:        fc.UID,
			Controller: &controller,
		})
	}
	return nil, !contains
}

// check if the advertisement is interesting and set its status accordingly
func (r *AdvertisementReconciler) CheckAdvertisement(adv *protocolv1.Advertisement) {
	// if announced resources are negative, always refuse the Adv
	for _, v := range adv.Spec.ResourceQuota.Hard {
		if v.Value() < 0 {
			adv.Status.AdvertisementStatus = AdvertisementRefused
			return
		}
	}

	switch r.ClusterConfig.IngoingConfig.AcceptPolicy {
	case policyv1.AutoAcceptWithinMaximum:
		if r.AcceptedAdvNum < r.ClusterConfig.IngoingConfig.MaxAcceptableAdvertisement {
			// the adv accepted so far are less than the configured maximum
			adv.Status.AdvertisementStatus = AdvertisementAccepted
			r.AcceptedAdvNum++
		} else {
			// the maximum has been reached: cannot accept
			adv.Status.AdvertisementStatus = AdvertisementRefused
		}
	case policyv1.ManualAccept:
		//TODO: manual accept/refuse, now we refuse all
		adv.Status.AdvertisementStatus = AdvertisementRefused
	}
}

func (r *AdvertisementReconciler) UpdateAdvertisement(adv *protocolv1.Advertisement) {
	if adv.Status.AdvertisementStatus == AdvertisementAccepted {
		metav1.SetMetaDataAnnotation(&adv.ObjectMeta, "advertisementStatus", "accepted")
		r.recordEvent("Advertisement "+adv.Name+" accepted", "Normal", "AdvertisementAccepted", adv)
	} else if adv.Status.AdvertisementStatus == AdvertisementRefused {
		metav1.SetMetaDataAnnotation(&adv.ObjectMeta, "advertisementStatus", "refused")
		r.recordEvent("Advertisement "+adv.Name+" refused", "Normal", "AdvertisementRefused", adv)
	}
	if err := r.Status().Update(context.Background(), adv); err != nil {
		klog.Error(err)
	}
}

func (r *AdvertisementReconciler) createVirtualKubelet(ctx context.Context, adv *protocolv1.Advertisement) error {

	name := "liqo-" + adv.Spec.ClusterId
	// Create the base resources
	vkSa := &v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       r.KubeletNamespace,
			OwnerReferences: pkg.GetOwnerReference(adv),
		},
	}
	err := pkg.CreateOrUpdate(r.Client, ctx, vkSa)
	if err != nil {
		return err
	}
	vkCrb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			OwnerReferences: pkg.GetOwnerReference(adv),
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", APIGroup: "", Name: name, Namespace: r.KubeletNamespace},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	}
	err = pkg.CreateOrUpdate(r.Client, ctx, vkCrb)
	if err != nil {
		return err
	}
	// Create the virtual Kubelet
	deploy := pkg.CreateVkDeployment(adv, name, r.KubeletNamespace, r.VKImage, r.InitVKImage, r.HomeClusterId)
	err = pkg.CreateOrUpdate(r.Client, ctx, deploy)
	if err != nil {
		return err
	}

	r.recordEvent("launching virtual-kubelet for cluster "+adv.Spec.ClusterId, "Normal", "VkCreated", adv)
	adv.Status.VkCreated = true
	adv.Status.VkReference = object_references.DeploymentReference{
		Namespace: deploy.Namespace,
		Name:      deploy.Name,
	}
	if err := r.Status().Update(ctx, adv); err != nil {
		klog.Error(err)
	}
	return nil
}

func (r *AdvertisementReconciler) recordEvent(msg string, eventType string, eventReason string, adv *protocolv1.Advertisement) {
	klog.Info(msg)
	r.EventsRecorder.Event(adv, eventType, eventReason, msg)
}

func (r *AdvertisementReconciler) cleanOldAdvertisements() {
	var advList protocolv1.AdvertisementList
	// every 10 minutes list advertisements and deletes the expired ones
	for {
		if err := r.Client.List(context.Background(), &advList, &client.ListOptions{}); err != nil {
			klog.Error(err)
			continue
		}
		for i := range advList.Items {
			adv := advList.Items[i]
			now := metav1.NewTime(time.Now())
			if adv.Spec.TimeToLive.Before(now.DeepCopy()) {
				if err := r.Client.Delete(context.Background(), &adv, &client.DeleteOptions{}); err != nil {
					klog.Error(err)
				}
				klog.Infof("Adv %v expired. TimeToLive was %v", adv.Name, adv.Spec.TimeToLive)
			}
		}
		time.Sleep(10 * time.Minute)
	}
}

func (r *AdvertisementReconciler) checkClusterStatus(adv protocolv1.Advertisement) error {
	// get the kubeconfig provided by the foreign cluster
	remoteKubeconfig, err := r.AdvClient.Client().CoreV1().Secrets(adv.Spec.KubeConfigRef.Namespace).Get(context.Background(), adv.Spec.KubeConfigRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	remoteClient, err := protocolv1.CreateAdvertisementClient("", remoteKubeconfig)
	if err != nil {
		return err
	}
	retry := 0
	for {
		_, err = remoteClient.Client().CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			retry++
		} else {
			retry = 0
		}
		if retry == int(r.ClusterConfig.KeepaliveThreshold) {
			return err
		}
		time.Sleep(time.Duration(r.ClusterConfig.KeepaliveRetryTime) * time.Second)
	}
}
