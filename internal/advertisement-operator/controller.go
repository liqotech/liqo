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

package advertisementOperator

import (
	"context"
	goerrors "errors"
	"fmt"
	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/internal/monitoring"
	advpkg "github.com/liqotech/liqo/pkg/advertisement-operator"
	"github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/discovery"
	objectreferences "github.com/liqotech/liqo/pkg/object-references"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/slice"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"sync"
	"time"
)

const FinalizerString = "advertisement.sharing.liqo.io/virtual-kubelet"

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
	ClusterConfig      configv1alpha1.AdvertisementConfig
	AdvClient          *crdClient.CRDClient
	DiscoveryClient    *crdClient.CRDClient
	RetryTimeout       time.Duration
	garbaceCollector   sync.Once
	checkRemoteCluster map[string]*sync.Once
}

// +kubebuilder:rbac:groups=sharing.liqo.io,resources=advertisements,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=advertisements/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;update;patch

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests/approval,verbs=get;update;patch
// +kubebuilder:rbac:groups=certificates.k8s.io,resourceNames=kubernetes.io/legacy-unknown,resources=signers,verbs=approve
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

func (r *AdvertisementReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	monitoring.GetPeeringProcessMonitoring().Start()

	// start the advertisement garbage collector
	go r.garbaceCollector.Do(func() {
		r.cleanOldAdvertisements()
	})

	// initialize the checkRemoteCluster map
	if r.checkRemoteCluster == nil {
		r.checkRemoteCluster = make(map[string]*sync.Once)
	}

	// get advertisement
	var adv advtypes.Advertisement
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

	// if the Advertisement is deleting and it was in accepted state, we have to decrease the counter
	if r.isDeleting(&adv) && adv.Status.AdvertisementStatus == advtypes.AdvertisementAccepted {
		r.AcceptedAdvNum--
		klog.Infof("Currently accepted Advertisements: %v", r.AcceptedAdvNum)
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

	if adv.Status.AdvertisementStatus != advtypes.AdvertisementAccepted {
		klog.Info("Advertisement " + adv.Name + " refused")
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
	}

	if !adv.Status.VkCreated {
		monitoring.GetPeeringProcessMonitoring().EventRegister(monitoring.AdvertisementOperator, monitoring.CreateVirtualKubelet, monitoring.Start)

		err := r.createVirtualKubelet(ctx, &adv)
		if err != nil {
			klog.Errorf("un error occurred while creating the virtual kubelet deployment: %v", err)
			return ctrl.Result{}, err
		}
		klog.Info("Correct creation of virtual kubelet deployment for cluster " + adv.Spec.ClusterId)

		// add finalizer
		if !slice.ContainsString(adv.Finalizers, FinalizerString, nil) {
			adv.Finalizers = append(adv.Finalizers, FinalizerString)
		}
		err = r.Update(ctx, &adv)
		if err != nil {
			return ctrl.Result{}, err
		}
		// start the keepalive check for the new cluster
		r.checkRemoteCluster[adv.Spec.ClusterId] = new(sync.Once)
		go r.checkRemoteCluster[adv.Spec.ClusterId].Do(func() {
			err := r.checkClusterStatus(adv)
			// if everything works well, the check is an infinite loop
			// therefore, if an error is returned, the foreign cluster is not reachable anymore
			if err != nil {
				// the foreign cluster is down: set adv status to trigger the unjoin
				klog.Error(err)
				if err2 := r.Delete(context.Background(), &adv); err2 != nil {
					klog.Error(err2)
				}
			}
		})
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
	}

	return ctrl.Result{}, nil
}

func (r *AdvertisementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&advtypes.Advertisement{}).
		Complete(r)
}

// checks that the Advertisement is being deleted
func (r *AdvertisementReconciler) isDeleting(adv *advtypes.Advertisement) bool {
	return !adv.DeletionTimestamp.IsZero()
}

// set Advertisement reference in related ForeignCluster
func (r *AdvertisementReconciler) UpdateForeignCluster(adv *advtypes.Advertisement) (error, bool) {
	tmp, err := r.DiscoveryClient.Resource("foreignclusters").List(metav1.ListOptions{
		LabelSelector: strings.Join([]string{
			discovery.ClusterIdLabel,
			adv.Spec.ClusterId,
		}, "="),
	})
	if err != nil {
		klog.Error(err, err.Error())
		return err, false
	}
	fcList, ok := tmp.(*discoveryv1alpha1.ForeignClusterList)
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
			APIVersion: fmt.Sprintf("%s/%s", discoveryv1alpha1.GroupVersion.Group, discoveryv1alpha1.GroupVersion.Version),
			Kind:       "ForeignCluster",
			Name:       fc.Name,
			UID:        fc.UID,
			Controller: &controller,
		})
	}
	return nil, !contains
}

// check if the advertisement is interesting and set its status accordingly
func (r *AdvertisementReconciler) CheckAdvertisement(adv *advtypes.Advertisement) {
	// if announced resources are negative, always refuse the Adv
	for _, v := range adv.Spec.ResourceQuota.Hard {
		if v.Value() < 0 {
			adv.Status.AdvertisementStatus = advtypes.AdvertisementRefused
			return
		}
	}

	switch r.ClusterConfig.IngoingConfig.AcceptPolicy {
	case configv1alpha1.AutoAcceptMax:
		if r.AcceptedAdvNum < r.ClusterConfig.IngoingConfig.MaxAcceptableAdvertisement {
			// the adv accepted so far are less than the configured maximum
			adv.Status.AdvertisementStatus = advtypes.AdvertisementAccepted
			r.AcceptedAdvNum++
		} else {
			// the maximum has been reached: cannot accept
			adv.Status.AdvertisementStatus = advtypes.AdvertisementRefused
		}
	case configv1alpha1.ManualAccept:
		//TODO: manual accept/refuse, now we refuse all
		adv.Status.AdvertisementStatus = advtypes.AdvertisementRefused
	}
}

func (r *AdvertisementReconciler) UpdateAdvertisement(adv *advtypes.Advertisement) {
	if adv.Status.AdvertisementStatus == advtypes.AdvertisementAccepted {
		metav1.SetMetaDataAnnotation(&adv.ObjectMeta, "advertisementStatus", "accepted")
		r.recordEvent("Advertisement "+adv.Name+" accepted", "Normal", "AdvertisementAccepted", adv)
	} else if adv.Status.AdvertisementStatus == advtypes.AdvertisementRefused {
		metav1.SetMetaDataAnnotation(&adv.ObjectMeta, "advertisementStatus", "refused")
		r.recordEvent("Advertisement "+adv.Name+" refused", "Normal", "AdvertisementRefused", adv)
	}
	if err := r.Status().Update(context.Background(), adv); err != nil {
		klog.Error(err)
	}
}

func (r *AdvertisementReconciler) createVirtualKubelet(ctx context.Context, adv *advtypes.Advertisement) error {

	secRef := adv.Spec.KubeConfigRef
	_, err := r.AdvClient.Client().CoreV1().Secrets(secRef.Namespace).Get(context.Background(), secRef.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		klog.Errorf("Cannot find secret %v in namespace %v for the virtual kubelet; error: %v", secRef.Name, secRef.Namespace, err)
		return err
	}
	name := virtualKubelet.VirtualKubeletPrefix + adv.Spec.ClusterId
	nodeName := virtualKubelet.VirtualNodePrefix + adv.Spec.ClusterId
	// Create the base resources
	vkSa := &v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       r.KubeletNamespace,
			OwnerReferences: advpkg.GetOwnerReference(adv),
		},
	}
	err = advpkg.CreateOrUpdate(r.Client, ctx, vkSa)
	if err != nil {
		return err
	}
	vkCrb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			OwnerReferences: advpkg.GetOwnerReference(adv),
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
	err = advpkg.CreateOrUpdate(r.Client, ctx, vkCrb)
	if err != nil {
		return err
	}
	// Create the virtual Kubelet
	deploy := advpkg.CreateVkDeployment(adv, name, r.KubeletNamespace, r.VKImage, r.InitVKImage, nodeName, r.HomeClusterId)
	err = advpkg.CreateOrUpdate(r.Client, ctx, deploy)
	if err != nil {
		return err
	}

	r.recordEvent("launching virtual-kubelet for cluster "+adv.Spec.ClusterId, "Normal", "VkCreated", adv)
	adv.Status.VkCreated = true
	adv.Status.VkReference = objectreferences.DeploymentReference{
		Namespace: deploy.Namespace,
		Name:      deploy.Name,
	}
	adv.Status.VnodeReference = objectreferences.NodeReference{
		Name: nodeName,
	}
	if err := r.Status().Update(ctx, adv); err != nil {
		klog.Error(err)
	} else {
		monitoring.GetPeeringProcessMonitoring().Complete(monitoring.AdvertisementOperator)
		monitoring.GetPeeringProcessMonitoring().EventRegister(monitoring.AdvertisementOperator, monitoring.CreateVirtualKubelet, monitoring.End)
	}
	return nil
}

func (r *AdvertisementReconciler) recordEvent(msg string, eventType string, eventReason string, adv *advtypes.Advertisement) {
	klog.Info(msg)
	r.EventsRecorder.Event(adv, eventType, eventReason, msg)
}

func (r *AdvertisementReconciler) cleanOldAdvertisements() {
	var advList advtypes.AdvertisementList
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
				// gracefully delete the Advertisement
				if err := r.Delete(context.Background(), &adv); err != nil {
					klog.Error(err)
				}
				klog.Infof("Adv %v expired. TimeToLive was %v", adv.Name, adv.Spec.TimeToLive)
			}
		}
		time.Sleep(10 * time.Minute)
	}
}

func (r *AdvertisementReconciler) checkClusterStatus(adv advtypes.Advertisement) error {
	// get the kubeconfig provided by the foreign cluster
	remoteKubeconfig, err := r.AdvClient.Client().CoreV1().Secrets(adv.Spec.KubeConfigRef.Namespace).Get(context.Background(), adv.Spec.KubeConfigRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	remoteClient, err := advtypes.CreateAdvertisementClient("", remoteKubeconfig, true, nil)
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
