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

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	"github.com/liqoTech/liqo/pkg/clusterConfig"
	"github.com/liqoTech/liqo/pkg/crdClient"
	liqonetOperator "github.com/liqoTech/liqo/pkg/liqonet"
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"net"
	"os"
	"time"
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	liqonetv1 "github.com/liqoTech/liqo/api/tunnel-endpoint/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	tunEndpointNameSuffix = "-tunendpoint"
	defualtPodCIDRValue   = "None"
)

// AdvertisementReconciler reconciles a Advertisement object
type TunnelEndpointCreator struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	ReservedSubnets map[string]*net.IPNet
	RetryTimeout      time.Duration
	IPManager       liqonetOperator.IpManager
	Mutex           sync.Mutex
	IsConfigured    bool
}

// +kubebuilder:rbac:groups=protocol.liqo.io,resources=advertisements,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=protocol.liqo.io,resources=advertisements/status,verbs=get;update;patch

//rbac for the liqonet.liqo.io api
// +kubebuilder:rbac:groups=liqonet.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=liqonet.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch

func (r *TunnelEndpointCreator) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("tunnelEndpointCreator-controller", req.NamespacedName)
	tunnelEndpointCreatorFinalizer := "tunnelEndpointCreator-Finalizer.liqonet.liqo.io"
	// get advertisement
	var adv protocolv1.Advertisement
	if err := r.Get(ctx, req.NamespacedName, &adv); err != nil {
		// reconcile was triggered by a delete request
		log.Info("Advertisement " + req.Name + " deleted")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// examine DeletionTimestamp to determine if object is under deletion
	if adv.ObjectMeta.DeletionTimestamp.IsZero() {
		if !liqonetOperator.ContainsString(adv.ObjectMeta.Finalizers, tunnelEndpointCreatorFinalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			adv.ObjectMeta.Finalizers = append(adv.Finalizers, tunnelEndpointCreatorFinalizer)
			if err := r.Update(ctx, &adv); err != nil {
				//while updating we check if the a resource version conflict happened
				//which means the version of the object we have is outdated.
				//a solution could be to return an error and requeue the object for later process
				//but if the object has been changed by another instance of the controller running in
				//another host it already has been put in the working queue so decide to forget the
				//current version and process the next item in the queue assured that we handle the object later
				if apierrors.IsConflict(err) {
					return ctrl.Result{}, nil
				}
				log.Error(err, "unable to update adv", "adv", adv.Name)
				return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
			}
			return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
		}
	} else {
		//the object is being deleted
		if liqonetOperator.ContainsString(adv.Finalizers, tunnelEndpointCreatorFinalizer) {
			if err := r.deleteTunEndpoint(&adv); err != nil {
				log.Error(err, "error while deleting endpoint")
				return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
			}

			//remove the finalizer from the list and update it.
			adv.Finalizers = liqonetOperator.RemoveString(adv.Finalizers, tunnelEndpointCreatorFinalizer)
			if err := r.Update(ctx, &adv); err != nil {
				if apierrors.IsConflict(err) {
					return ctrl.Result{}, nil
				}
				log.Error(err, "unable to update adv %s", adv.Name, "in namespace", adv.Namespace)
				return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
			}
		}
		//remove the reserved ip for the cluster
		r.IPManager.RemoveReservedSubnet(adv.Spec.ClusterId)
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
	}

	err := r.createOrUpdateTunEndpoint(&adv)
	if err != nil {
		log.Error(err, "error while creating endpoint")
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
	}
	return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
}

func (r *TunnelEndpointCreator) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&protocolv1.Advertisement{}).Owns(&liqonetv1.TunnelEndpoint{}).
		Complete(r)
}

func (r *TunnelEndpointCreator) getNamespace() (namespace string, err error) {
	//it is passed to the pod during the deployment; in its manifest
	keyNamespace := "POD_NAMESPACE"
	namespace, found := os.LookupEnv(keyNamespace)
	if !found {
		return namespace, errors.New("the environment variable " + keyNamespace + "is not set. ")
	}
	return namespace, nil
}

func (r *TunnelEndpointCreator) getTunnelEndpointByKey(key types.NamespacedName) (tunEndpoint *liqonetv1.TunnelEndpoint, err error) {
	ctx := context.Background()
	tunEndpoint = new(liqonetv1.TunnelEndpoint)
	err = r.Get(ctx, key, tunEndpoint)
	if err != nil {
		return tunEndpoint, err
	}
	return tunEndpoint, err
}

func (r *TunnelEndpointCreator) getTunnelEndpointByName(name string) (tunEndpoint *liqonetv1.TunnelEndpoint, err error) {
	ctx := context.Background()
	//create the key to be used to retrieve the CR
	namespace, err := r.getNamespace()
	if err != nil {
		return tunEndpoint, err
	}
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	//try to retrieve the CR
	err = r.Get(ctx, key, tunEndpoint)
	if err != nil {
		return tunEndpoint, err
	}
	return tunEndpoint, err
}

func (r *TunnelEndpointCreator) getTunEndPerADV(adv *protocolv1.Advertisement) (liqonetv1.TunnelEndpoint, error) {
	ctx := context.Background()
	var tunEndpoint liqonetv1.TunnelEndpoint
	//build the key used to retrieve the tunnelEndpoint CR
	tunEndKey := types.NamespacedName{
		Namespace: adv.Namespace,
		Name:      adv.Spec.ClusterId + tunEndpointNameSuffix,
	}
	//retrieve the tunnelEndpoint CR
	err := r.Get(ctx, tunEndKey, &tunEndpoint)
	//if the tunEndpoint CR can not be retrieved then return the error
	//if this come here it means that the CR has been created because the function is called only if the create process goes well
	return tunEndpoint, err
}

func (r *TunnelEndpointCreator) isTunEndpointUpdated(adv *protocolv1.Advertisement, tunEndpoint *liqonetv1.TunnelEndpoint) bool {
	if adv.Spec.ClusterId == tunEndpoint.Spec.ClusterID && adv.Spec.Network.PodCIDR == tunEndpoint.Spec.PodCIDR && adv.Spec.Network.GatewayIP == tunEndpoint.Spec.TunnelPublicIP && adv.Spec.Network.GatewayPrivateIP == tunEndpoint.Spec.TunnelPrivateIP {
		return true
	} else {
		return false
	}
}

func (r *TunnelEndpointCreator) createOrUpdateTunEndpoint(adv *protocolv1.Advertisement) error {
	_, err := r.getTunEndPerADV(adv)
	if err == nil {
		err := r.updateTunEndpoint(adv)
		if err == nil {
			return nil
		} else {
			return err
		}

	} else if apierrors.IsNotFound(err) {
		err := r.createTunEndpoint(adv)
		if err == nil {
			return nil
		} else {
			return err
		}
	} else {
		return err
	}
}

func (r *TunnelEndpointCreator) updateTunEndpoint(adv *protocolv1.Advertisement) error {
	//funcName := "updateTunEndpoint"
	ctx := context.Background()
	//log := r.Log.WithValues("tunnelEndpointCreator-controller", funcName)
	var tunEndpoint liqonetv1.TunnelEndpoint
	var remoteRemappedPodCIDR string
	//build the key used to retrieve the tunnelEndpoint CR
	tunEndKey := types.NamespacedName{
		Namespace: adv.Namespace,
		Name:      adv.Spec.ClusterId + tunEndpointNameSuffix,
	}
	//retrieve the tunnelEndpoint CR
	err := r.Get(ctx, tunEndKey, &tunEndpoint)
	//if the tunEndpoint CR can not be retrieved then return the error
	//if this come here it means that the CR has been created because the function is called only if the create process goes well
	if err != nil {
		return err
	}

	if tunEndpoint.Status.Phase == "" {
		//check if the PodCidr of the remote cluster overlaps with any of the subnets on the local cluster
		_, subnet, err := net.ParseCIDR(adv.Spec.Network.PodCIDR)
		if err != nil {
			return fmt.Errorf("an error occured while parsing podCidr %s from adv %s :%v", adv.Spec.Network.PodCIDR, adv.Name, err)
		}
		subnet, err = r.IPManager.GetNewSubnetPerCluster(subnet, tunEndpoint.Spec.ClusterID)
		if err != nil {
			return err
		}
		if subnet != nil {
			remoteRemappedPodCIDR = subnet.String()
			//update adv status
			adv.Status.RemoteRemappedPodCIDR = remoteRemappedPodCIDR
			err := r.Status().Update(ctx, adv)
			if err != nil {
				return err
			}
			//update tunEndpoint status
			tunEndpoint.Status.RemoteRemappedPodCIDR = remoteRemappedPodCIDR
			tunEndpoint.Status.Phase = "New"
			err = r.Status().Update(ctx, &tunEndpoint)
			if err != nil {
				return err
			}
		} else {
			//update adv status
			adv.Status.RemoteRemappedPodCIDR = defualtPodCIDRValue
			err := r.Status().Update(ctx, adv)
			if err != nil {
				return err
			}
			//update tunEndpoint status
			tunEndpoint.Status.RemoteRemappedPodCIDR = defualtPodCIDRValue
			tunEndpoint.Status.Phase = "New"
			err = r.Status().Update(ctx, &tunEndpoint)
			if err != nil {
				return err
			}
		}
		return nil
	} else if tunEndpoint.Status.Phase == "New" {
		if adv.Status.LocalRemappedPodCIDR == "" {
			return nil
		} else {
			tunEndpoint.Status.LocalRemappedPodCIDR = adv.Status.LocalRemappedPodCIDR
			tunEndpoint.Status.Phase = "Processed"
			err = r.Status().Update(ctx, &tunEndpoint)
			if err != nil {
				return err
			}
		}
		return nil
	} else {
		return nil
	}
}

//we do not support updates to the ADV CR by the user, at least not yet
//we assume that the ADV is created by the Remote Server and is the only one who can remove or update it
//if the ADV has to be updated first we remove it and then recreate the it with new values
//For each ADV CR a tunnelEndpoint CR is created in the same namespace as the ADV CR and the name of the later
//is derived from the name of the ADV adding a suffix. Doing so, given an ADV we can always create, update or delete
//the associated tunnelEndpoint CR without the necessity for its key to be saved by the operator.
func (r *TunnelEndpointCreator) createTunEndpoint(adv *protocolv1.Advertisement) error {
	funcName := "createTunEndpoint"
	ctx := context.Background()
	log := r.Log.WithValues("tunnelEndpointCreator-controller", funcName)
	var tunEndpoint liqonetv1.TunnelEndpoint

	//build the key used to retrieve the tunnelEndpoint CR
	tunEndKey := types.NamespacedName{
		Namespace: adv.Namespace,
		Name:      adv.Spec.ClusterId + tunEndpointNameSuffix,
	}
	//retrieve the tunnelEndpoint CR
	err := r.Get(ctx, tunEndKey, &tunEndpoint)
	//if the CR exist then do nothing and return
	if err == nil {
		return nil
	} else if apierrors.IsNotFound(err) {
		//if tunnelEndpoint referenced by the key does not exist then we create it
		tunEndpoint := &liqonetv1.TunnelEndpoint{
			ObjectMeta: v1.ObjectMeta{
				//the name is derived from the clusterID
				Name: adv.Spec.ClusterId + tunEndpointNameSuffix,
				//the namespace is read from the Environment variable passe to the pod
				Namespace: adv.Namespace,
			},
			Spec: liqonetv1.TunnelEndpointSpec{
				ClusterID:       adv.Spec.ClusterId,
				PodCIDR:         adv.Spec.Network.PodCIDR,
				TunnelPublicIP:  adv.Spec.Network.GatewayIP,
				TunnelPrivateIP: adv.Spec.Network.GatewayPrivateIP,
			},
			Status: liqonetv1.TunnelEndpointStatus{},
		}
		var ownerReferences []v1.OwnerReference
		var controller bool = true
		tunEndpoint.SetOwnerReferences(append(ownerReferences, v1.OwnerReference{
			APIVersion: adv.APIVersion,
			Kind:       adv.Kind,
			Name:       adv.Name,
			UID:        adv.UID,
			Controller: &controller,
		}))
		err = r.Create(ctx, tunEndpoint)
		if err != nil {
			log.Info("failed to create the custom resource", "name", tunEndpoint.Name, "namespace", tunEndpoint.Namespace, "clusterId", adv.Spec.ClusterId, "podCIDR", adv.Spec.Network.PodCIDR, "gatewayPublicIP", adv.Spec.Network.GatewayIP, "tunnelPrivateIP", adv.Spec.Network.GatewayPrivateIP)
			return err
		}
		log.Info("created the custom resource", "name", tunEndpoint.Name, "namespace", tunEndpoint.Namespace, "clusterId", adv.Spec.ClusterId, "podCIDR", adv.Spec.Network.PodCIDR, "gatewayPublicIP", adv.Spec.Network.GatewayIP, "tunnelPrivateIP", adv.Spec.Network.GatewayPrivateIP)
		return nil
	} else {
		return err
	}
}

func (r *TunnelEndpointCreator) deleteTunEndpoint(adv *protocolv1.Advertisement) error {
	ctx := context.Background()
	var tunEndpoint liqonetv1.TunnelEndpoint
	//build the key used to retrieve the tunnelEndpoint CR
	tunEndKey := types.NamespacedName{
		Namespace: adv.Namespace,
		Name:      adv.Spec.ClusterId + tunEndpointNameSuffix,
	}
	//retrieve the tunnelEndpoint CR
	err := r.Get(ctx, tunEndKey, &tunEndpoint)
	//if the CR exist then do nothing and return
	if err == nil {
		err := r.Delete(ctx, &tunEndpoint)
		if err != nil {
			return fmt.Errorf("unable to delete endpoint %s in namespace %s : %v", tunEndpoint.Name, tunEndpoint.Namespace, err)
		} else {
			return nil
		}
	} else if apierrors.IsNotFound(err) {
		return nil
	} else {
		return fmt.Errorf("unable to get endpoint with key %s: %v", tunEndKey.String(), err)
	}
}

func (r *TunnelEndpointCreator) WatchConfiguration(config *rest.Config, gv *schema.GroupVersion) {
	config.ContentConfig.GroupVersion = gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	CRDclient, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	go clusterConfig.WatchConfiguration(func(configuration *policyv1.ClusterConfig) {

		//this section is executed at start-up time
		if !r.IsConfigured {
			if err := r.InitConfiguration(configuration); err != nil{
				return
			}
		}

	}, CRDclient, "")
}

func (r *TunnelEndpointCreator) InitConfiguration(config *policyv1.ClusterConfig) error {
	var isError = false
	//get the reserved subnets from che configuration CRD
	reservedSubnets, err := r.GetConfiguration(config)
	if err != nil {
		klog.Error(err)
		return err
	}
	//get subnets used by foreign clusters
	clusterSubnets, err := r.GetClustersSubnets()
	if err != nil {
		klog.Error(err)
		return err
	}
	//here we check that there are no conflicts between the configuration and the already used subnets
	if clusterSubnets != nil{
		for _, usedSubnet := range clusterSubnets {
			if liqonetOperator.VerifyNoOverlap(reservedSubnets, usedSubnet) {
				klog.Infof("there is a conflict between a reserved subnet given by the configuration and subnet used by another cluster. Please consider to remove the one of the conflicting subnets")
				isError = true
			}
		}
	}
	//if no conflicts or errors occurred then we start the IPAM
	if !isError {
		//here we acquire the lock of the mutex
		r.Mutex.Lock()
		if err := r.IPManager.Init(); err != nil {
			klog.Errorf("an error occurred while initializing the IP manager -> err")
			r.Mutex.Unlock()
			return err
		}
		//here we populate the used subnets with the reserved subnets and the subnets used by clusters
		for _, value := range reservedSubnets{
			r.IPManager.UsedSubnets [value.String()] = value
		}
		if clusterSubnets != nil{
			for _, value := range clusterSubnets{
				r.IPManager.UsedSubnets [value.String()] = value
			}
		}
		//we remove all the free subnets that have conflicts with the used subnets
		for _, net := range r.IPManager.FreeSubnets {
			if bool := liqonetOperator.VerifyNoOverlap(r.IPManager.UsedSubnets, net); bool {
				delete(r.IPManager.FreeSubnets, net.String())
				//we add it to a new map, if the reserved ip is removed from the config then the conflicting subnets can be inserted in the free pool of subnets
				r.IPManager.ConflictingSubnets[net.String()] = net
				klog.Infof("removing subnet %s from the free pool", net.String())
			}
		}
		r.IsConfigured = true
		r.ReservedSubnets = reservedSubnets
		r.Mutex.Unlock()
	}else{
		return fmt.Errorf("There are conflicts between the reserved subnets given in the configuration and the already used subnets in the tunnelEndpoint CRs.")
	}
	return nil
}

func (r *TunnelEndpointCreator) UpdateConfiguration(config *policyv1.ClusterConfig) error{
	var addedSubnets, removedSubnets map[string]*net.IPNet
	//get the reserved subnets from che configuration CRD
	reservedSubnets, err := r.GetConfiguration(config)
	if err != nil {
		klog.Error(err)
		return err
	}

}

func (r *TunnelEndpointCreator) GetConfiguration(config *policyv1.ClusterConfig) (map[string]*net.IPNet, error) {
	correctlyParsed := true
	reservedSubnets := make(map[string]*net.IPNet)
	liqonetConfig := config.Spec.LiqonetConfig
	//check that the reserved subnets are in the right format
	for _, subnet := range liqonetConfig.ReservedSubnets {
		_, sn, err := net.ParseCIDR(subnet)
		if err != nil {
			klog.Errorf("an error occurred while parsing configuration: %s", err)
			correctlyParsed = false
		} else {
			klog.Infof("subnet %s correctly added to the reserved subnets", sn.String())
			reservedSubnets[sn.String()] = sn
		}
	}
	if !correctlyParsed {
		return nil, fmt.Errorf("the reserved subnets list is not in the correct format")
	}
	return reservedSubnets, nil
}

//it returns the subnets used by the foreign clusters
func (r *TunnelEndpointCreator) GetClustersSubnets() (map[string]*net.IPNet, error) {
	ctx := context.Background()
	var tunEndList liqonetv1.TunnelEndpointList
	subnets := make(map[string]*net.IPNet)
	err := r.Client.List(ctx, &tunEndList, &client.ListOptions{})
	if err != nil {
		klog.Errorf("unable to get the list of tunnelEndpoint custom resources -> %s", err)
		return nil, err
	}
	//if the list is empty return a nil slice and nil error
	if tunEndList.Items == nil {
		return nil, nil
	}
	for _, tunEnd := range tunEndList.Items {
		if tunEnd.Status.LocalRemappedPodCIDR != "" && tunEnd.Status.LocalRemappedPodCIDR != defualtPodCIDRValue {
			_, sn, err := net.ParseCIDR(tunEnd.Status.LocalRemappedPodCIDR)
			if err != nil {
				klog.Errorf("an error occurred while parsing configuration: %s", err)
				return nil, err
			}
			subnets[sn.String()] = sn
			klog.Infof("subnet %s already reserved for cluster %s", tunEnd.Status.LocalRemappedPodCIDR, tunEnd.Spec.ClusterID)
		} else if tunEnd.Status.LocalRemappedPodCIDR == defualtPodCIDRValue {
			_, sn, err := net.ParseCIDR(tunEnd.Spec.PodCIDR)
			if err != nil {
				klog.Errorf("an error occurred while parsing configuration: %s", err)
				return nil, err
			}
			subnets[sn.String()] = sn
			klog.Infof("subnet %s already reserved for cluster %s", tunEnd.Spec.PodCIDR, tunEnd.Spec.ClusterID)
		} else {
		}
	}
	return subnets, nil
}
