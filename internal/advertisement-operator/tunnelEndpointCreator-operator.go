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
	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"

	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	protocolv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	dronetv1 "github.com/netgroup-polito/dronev2/api/tunnel-endpoint/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// AdvertisementReconciler reconciles a Advertisement object
type TunnelEndpointCreator struct {
	client.Client
	Log               logr.Logger
	Scheme            *runtime.Scheme
	TunnelEndpointMap map[string]types.NamespacedName
}

// +kubebuilder:rbac:groups=protocol.drone.com,resources=advertisements,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=protocol.drone.com,resources=advertisements/status,verbs=get;update;patch

//rbac for the dronet.drone.com api
// +kubebuilder:rbac:groups=dronet.drone.com,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dronet.drone.com,resources=tunnelendpoints/status,verbs=get;update;patch

func (r *TunnelEndpointCreator) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("tunnelEndpointCreator-controller", req.NamespacedName)

	// get advertisement
	var adv protocolv1.Advertisement
	if err := r.Get(ctx, req.NamespacedName, &adv); err != nil {
		// reconcile was triggered by a delete request
		log.Info("Advertisement " + req.Name + " deleted")
		if client.IgnoreNotFound(err) == nil {
			err := r.deleteTunEndpoint(req.NamespacedName)
			if err != nil {
				log.Error(err, "failed to delete the endpointTunnel CR associated to the advertisment", "advKey", req.NamespacedName.String())
			}
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	err := r.createOrUpdateTunEndpoint(&adv, req.NamespacedName)
	if err != nil {
		log.Error(err, "error while creating or updating")
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func (r *TunnelEndpointCreator) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&protocolv1.Advertisement{}).
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

func (r *TunnelEndpointCreator) getTunnelEndpointByKey(key types.NamespacedName) (tunEndpoint *dronetv1.TunnelEndpoint, err error) {
	ctx := context.Background()
	tunEndpoint = new(dronetv1.TunnelEndpoint)
	err = r.Get(ctx, key, tunEndpoint)
	if err != nil {
		return tunEndpoint, err
	}
	return tunEndpoint, err
}

func (r *TunnelEndpointCreator) getTunnelEndpointByName(name string) (tunEndpoint *dronetv1.TunnelEndpoint, err error) {
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

func (r *TunnelEndpointCreator) getTunEndPerADV(adv *protocolv1.Advertisement) (tunEndpoint *dronetv1.TunnelEndpoint, err error) {
	//check if a tunnelEndpointCR has been created for the current ADV
	tunEndpointKey := adv.Status.TunnelEndpointKey
	if tunEndpointKey.Name != "" && tunEndpointKey.Namespace != "" {
		tunEndpoint, err := r.getTunnelEndpointByKey(types.NamespacedName(tunEndpointKey))
		if err == nil {
			return tunEndpoint, err
		} else if apierrors.IsNotFound(err) {
			return tunEndpoint, errors.New("notFound")
		} else {
			return tunEndpoint, err
		}
	} else {
		return tunEndpoint, errors.New("notFound")
	}
}

func (r *TunnelEndpointCreator) isTunEndpointUpdated(adv *protocolv1.Advertisement, tunEndpoint *dronetv1.TunnelEndpoint) bool {
	if adv.Spec.ClusterId == tunEndpoint.Spec.ClusterID && adv.Spec.Network.PodCIDR == tunEndpoint.Spec.PodCIDR && adv.Spec.Network.GatewayIP == tunEndpoint.Spec.GatewayPublicIP && adv.Spec.Network.GatewayPrivateIP == tunEndpoint.Spec.TunnelPrivateIP {
		return true
	} else {
		return false
	}
}

func (r *TunnelEndpointCreator) createOrUpdateTunEndpoint(adv *protocolv1.Advertisement, advKey types.NamespacedName) error {
	tunEndpoint, err := r.getTunEndPerADV(adv)
	if err == nil {
		equal := r.isTunEndpointUpdated(adv, tunEndpoint)
		if equal {
			return nil
		} else {
			err := r.updateTunEndpoint(adv, advKey, tunEndpoint)
			if err == nil {
				return nil
			} else {
				return err
			}
		}
	} else if err.Error() == "notFound" {
		err := r.createTunEndpoint(adv, advKey)
		if err == nil {
			return nil
		} else {
			return err
		}
	} else {
		return err
	}
}

func (r *TunnelEndpointCreator) updateTunEndpoint(adv *protocolv1.Advertisement, advKey types.NamespacedName, tunEndpoint *dronetv1.TunnelEndpoint) error {
	funcName := "updateTunEndpoint"
	ctx := context.Background()
	log := r.Log.WithValues("tunnelEndpointCreator-controller", funcName)
	tunEndpoint.Spec.ClusterID = adv.Spec.ClusterId
	tunEndpoint.Spec.PodCIDR = adv.Spec.Network.PodCIDR
	tunEndpoint.Spec.GatewayPublicIP = adv.Spec.Network.GatewayIP
	tunEndpoint.Spec.TunnelPrivateIP = adv.Spec.Network.GatewayPrivateIP
	err := r.Update(ctx, tunEndpoint)
	if err == nil {
		log.Info("updated the custom resource", "clusterId", adv.Spec.ClusterId, "podCIDR", adv.Spec.Network.PodCIDR, "gatewayPublicIP", adv.Spec.Network.GatewayIP, "tunnelPrivateIP", adv.Spec.Network.GatewayPrivateIP)
		adv.Status.TunnelEndpointKey = protocolv1.NamespacedName{
			Namespace: tunEndpoint.Namespace,
			Name:      tunEndpoint.Name,
		}
		err := r.Client.Status().Update(ctx, adv)
		if err == nil {
			log.Info("updated the status of ADV custom resource", "tunnelEndpointKey.Namespace", tunEndpoint.Namespace, "tunnelEndpointKey.Name", tunEndpoint.Name)
			//add the value to the map
			r.TunnelEndpointMap[advKey.String()] = types.NamespacedName(adv.Status.TunnelEndpointKey)
			return nil
		} else {
			log.Info("failed to update the status of ADV custom resource", "tunnelEndpointKey.Namespace", tunEndpoint.Namespace, "tunnelEndpointKey.Name", tunEndpoint.Name)
			return err
		}
	} else {
		log.Info("failed to update the custom resource", "clusterId", adv.Spec.ClusterId, "podCIDR", adv.Spec.Network.PodCIDR, "gatewayPublicIP", adv.Spec.Network.GatewayIP, "tunnelPrivateIP", adv.Spec.Network.GatewayPrivateIP)
		return err
	}
}

func (r *TunnelEndpointCreator) createTunEndpoint(adv *protocolv1.Advertisement, advKey types.NamespacedName) error {
	funcName := "createTunEndpoint"
	ctx := context.Background()
	log := r.Log.WithValues("tunnelEndpointCreator-controller", funcName)
	namespace, err := r.getNamespace()
	if err != nil {
		return err
	}
	//TODO: implement the IPAM algorithm
	tunEndpoint := &dronetv1.TunnelEndpoint{
		ObjectMeta: v1.ObjectMeta{
			//the name is derived from the clusterID
			Name: adv.Spec.ClusterId + "-tunnendpoint",
			//the namespace is read from the Environment variable passe to the pod
			Namespace: namespace,
		},
		Spec: dronetv1.TunnelEndpointSpec{
			ClusterID:       adv.Spec.ClusterId,
			PodCIDR:         adv.Spec.Network.PodCIDR,
			RemappedPodCIDR: "",
			GatewayPublicIP: adv.Spec.Network.GatewayIP,
			TunnelPrivateIP: adv.Spec.Network.GatewayPrivateIP,
			NATEnabled:      false,
		},
		Status: dronetv1.TunnelEndpointStatus{},
	}
	err = r.Create(ctx, tunEndpoint)
	if err == nil {
		log.Info("created the custom resource", "name", tunEndpoint.Name, "namespace", tunEndpoint.Namespace, "clusterId", adv.Spec.ClusterId, "podCIDR", adv.Spec.Network.PodCIDR, "gatewayPublicIP", adv.Spec.Network.GatewayIP, "tunnelPrivateIP", adv.Spec.Network.GatewayPrivateIP)
		adv.Status.TunnelEndpointKey = protocolv1.NamespacedName{
			Namespace: tunEndpoint.Namespace,
			Name:      tunEndpoint.Name,
		}
		//add the value to the map
		r.TunnelEndpointMap[advKey.String()] = types.NamespacedName(adv.Status.TunnelEndpointKey)
		err := r.Client.Status().Update(ctx, adv)
		if err == nil {
			log.Info("updated the status of ADV custom resource", "tunnelEndpointKey.Namespace", tunEndpoint.Namespace, "tunnelEndpointKey.Name", tunEndpoint.Name)
			return nil
		} else {
			log.Info("failed to update the status of ADV custom resource", "tunnelEndpointKey.Namespace", tunEndpoint.Namespace, "tunnelEndpointKey.Name", tunEndpoint.Name)
			return err
		}
	} else {
		log.Info("failed to create the custom resource", "name", tunEndpoint.Name, "namespace", tunEndpoint.Namespace, "clusterId", adv.Spec.ClusterId, "podCIDR", adv.Spec.Network.PodCIDR, "gatewayPublicIP", adv.Spec.Network.GatewayIP, "tunnelPrivateIP", adv.Spec.Network.GatewayPrivateIP)
		return err
	}
}

func (r *TunnelEndpointCreator) deleteTunEndpoint(advKey types.NamespacedName) error {
	funcName := "deleteTunEndpoint"
	ctx := context.Background()
	log := r.Log.WithValues("tunnelEndpointCreator-controller", funcName)
	tunEndpointKey, found := r.TunnelEndpointMap[advKey.String()]
	if found {
		tunEndpoint, err := r.getTunnelEndpointByKey(tunEndpointKey)
		if err == nil {
			err := r.Delete(ctx, tunEndpoint)
			if err == nil {
				log.Info("deleted tunnelEndopoint", "name", tunEndpoint.Name)
				//remove the entry from the map
				delete(r.TunnelEndpointMap, advKey.String())
				return nil
			} else {
				log.Info("failed to delete tunnelEndopoint", "name", tunEndpoint.Name)
				return err
			}
		} else if apierrors.IsNotFound(err) {
			log.Info("the required tunnelEndpoint CR is not found", "key", tunEndpointKey.String())
			//remove the entry from the map
			delete(r.TunnelEndpointMap, advKey.String())
			return nil
		} else {
			log.Info("failed to retrieve the tunnelEndopint CR", "key", tunEndpointKey.String())
			return err
		}
	} else {
		log.Info("no tunnelEndpoint CR related to adv was found", "advKey", advKey.String())
		return nil
	}
}
