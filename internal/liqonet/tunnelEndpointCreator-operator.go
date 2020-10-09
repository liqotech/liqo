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

package liqonetOperators

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator"
	liqonetOperator "github.com/liqotech/liqo/pkg/liqonet"
	"github.com/liqotech/liqo/pkg/owner"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"k8s.io/utils/pointer"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	TunEndpointNamePrefix = "tun-endpoint-"
	NetConfigNamePrefix   = "net-config-"
	defaultPodCIDRValue   = "None"
)

var (
	ResyncPeriod = 30 * time.Second

	ForeignClusterGVR = schema.GroupVersionResource{
		Group:    discoveryv1alpha1.GroupVersion.Group,
		Version:  discoveryv1alpha1.GroupVersion.Version,
		Resource: "foreignclusters",
	}
	result = ctrl.Result{
		Requeue:      false,
		RequeueAfter: 5 * time.Second,
	}
)

type networkParam struct {
	remoteClusterID  string
	remoteGatewayIP  string
	remotePodCIDR    string
	remoteNatPodCIDR string
	localGatewayIP   string
	localNatPodCIDR  string
}

type TunnelEndpointCreator struct {
	client.Client
	Log                        logr.Logger
	Scheme                     *runtime.Scheme
	DynClient                  dynamic.Interface
	DynFactory                 dynamicinformer.DynamicSharedInformerFactory
	GatewayIP                  string
	PodCIDR                    string
	ServiceCIDR                string
	netParamPerCluster         map[string]networkParam
	ReservedSubnets            map[string]*net.IPNet
	IPManager                  liqonetOperator.IpManager
	Mutex                      sync.Mutex
	IsConfigured               bool
	Configured                 chan bool
	ForeignClusterStartWatcher chan bool
	ForeignClusterStopWatcher  chan struct{}
	RunningWatchers            bool
	RetryTimeout               time.Duration
}

//rbac for the net.liqo.io api
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch

func (r *TunnelEndpointCreator) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	if !r.IsConfigured {
		<-r.Configured
		klog.Infof("operator configured")
	}
	ctx := context.Background()
	tunnelEndpointCreatorFinalizer := "tunnelEndpointCreator-Finalizer.liqonet.liqo.io"
	// get networkConfig
	var netConfig netv1alpha1.NetworkConfig
	if err := r.Get(ctx, req.NamespacedName, &netConfig); apierrors.IsNotFound(err) {
		// reconcile was triggered by a delete request
		return ctrl.Result{}, client.IgnoreNotFound(err)
	} else if err != nil {
		klog.Errorf("an error occurred while getting resource %s: %s", req.NamespacedName, err)
		return result, err
	}
	// examine DeletionTimestamp to determine if object is under deletion
	if netConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		if !liqonetOperator.ContainsString(netConfig.ObjectMeta.Finalizers, tunnelEndpointCreatorFinalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			netConfig.ObjectMeta.Finalizers = append(netConfig.Finalizers, tunnelEndpointCreatorFinalizer)
			if err := r.Update(ctx, &netConfig); err != nil {
				//while updating we check if the a resource version conflict happened
				//which means the version of the object we have is outdated.
				//a solution could be to return an error and requeue the object for later process
				//but if the object has been changed by another instance of the controller running in
				//another host it already has been put in the working queue so decide to forget the
				//current version and process the next item in the queue assured that we handle the object later
				if apierrors.IsConflict(err) {
					return ctrl.Result{}, nil
				}
				klog.Errorf("an error occurred while setting finalizer for resource %s: %s", req.NamespacedName, err)
				return result, err
			}
			return result, nil
		}
	} else {
		//the object is being deleted
		if err := r.deleteTunEndpoint(&netConfig); err != nil {
			klog.Errorf("an error occurred while deleting tunnel endpoint related to %s: %s", netConfig.Name, err)
			return result, err
		}
		if liqonetOperator.ContainsString(netConfig.Finalizers, tunnelEndpointCreatorFinalizer) {
			//remove the finalizer from the list and update it.
			netConfig.Finalizers = liqonetOperator.RemoveString(netConfig.Finalizers, tunnelEndpointCreatorFinalizer)
			if err := r.Update(ctx, &netConfig); err != nil {
				if apierrors.IsConflict(err) {
					return ctrl.Result{}, nil
				}
				klog.Errorf("an error occurred while removing finalizer from resource %s: %s", req.NamespacedName, err)
				return result, err
			}
		}
		//remove the reserved ip for the cluster
		r.IPManager.RemoveReservedSubnet(netConfig.Spec.ClusterID)
		return result, nil
	}

	//check if the netconfig is local or remote
	labels := netConfig.GetLabels()
	if val, ok := labels[crdReplicator.LocalLabelSelector]; ok && val == "true" {
		return result, r.processLocalNetConfig(&netConfig)
	}
	if _, ok := labels[crdReplicator.RemoteLabelSelector]; ok {
		return result, r.processRemoteNetConfig(&netConfig)
	}
	return result, nil
}

func (r *TunnelEndpointCreator) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1alpha1.NetworkConfig{}).
		Complete(r)
}

// SetupSignalHandlerForTunnelEndpointCreator registers for SIGTERM, SIGINT, SIGKILL. A stop channel is returned
// which is closed on one of these signals.
func (r *TunnelEndpointCreator) SetupSignalHandlerForTunEndCreator() (stopCh <-chan struct{}) {
	klog.Infof("starting signal handler for tunnelEndpointCreator-operator")
	stop := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, shutdownSignals...)
	go func(r *TunnelEndpointCreator) {
		sig := <-c
		klog.Infof("received signal: %s", sig.String())
		//closing shared informers
		close(r.ForeignClusterStopWatcher)
		close(stop)
	}(r)
	return stop
}

func (d *TunnelEndpointCreator) Watcher(sharedDynFactory dynamicinformer.DynamicSharedInformerFactory, resourceType schema.GroupVersionResource, handlerFuncs cache.ResourceEventHandlerFuncs, start chan bool, stopCh chan struct{}) {
	<-start
	dynInformer := sharedDynFactory.ForResource(resourceType)
	klog.Infof("starting watcher for resources %s", resourceType.String())
	//adding handlers to the informer
	dynInformer.Informer().AddEventHandler(handlerFuncs)
	dynInformer.Informer().Run(stopCh)
}

func (r *TunnelEndpointCreator) createNetConfig(fc *discoveryv1alpha1.ForeignCluster) error {
	clusterID := fc.Spec.ClusterIdentity.ClusterID
	netConfig := netv1alpha1.NetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: NetConfigNamePrefix,
			Labels: map[string]string{
				crdReplicator.LocalLabelSelector: "true",
				crdReplicator.DestinationLabel:   clusterID,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1alpha1",
					Kind:       "ForeignCluster",
					Name:       fc.Name,
					UID:        fc.UID,
					Controller: pointer.BoolPtr(true),
				},
			},
		},
		Spec: netv1alpha1.NetworkConfigSpec{
			ClusterID:      clusterID,
			PodCIDR:        r.PodCIDR,
			TunnelPublicIP: r.GatewayIP,
		},
		Status: netv1alpha1.NetworkConfigStatus{},
	}
	//check if the resource for the remote cluster already exists
	_, exists, err := r.GetNetworkConfig(clusterID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	err = r.Create(context.TODO(), &netConfig)
	if err != nil {
		klog.Errorf("an error occurred while creating resource %s of type %s: %s", netConfig.Name, netv1alpha1.GroupVersion.String(), err)
		return err
	}
	klog.Infof("resource %s of type %s created", netConfig.Name, netv1alpha1.GroupVersion.String())
	return nil

}

func (r *TunnelEndpointCreator) deleteNetConfig(fc *discoveryv1alpha1.ForeignCluster) error {
	clusterID := fc.Spec.ClusterIdentity.ClusterID
	netConfigList := &netv1alpha1.NetworkConfigList{}
	labels := client.MatchingLabels{crdReplicator.DestinationLabel: clusterID}
	err := r.List(context.Background(), netConfigList, labels)
	if err != nil {
		klog.Errorf("an error occurred while listing resources: %s", err)
		return err
	}
	if len(netConfigList.Items) != 1 {
		if len(netConfigList.Items) == 0 {
			klog.Infof("nothing to remove: a resource of type %s for remote cluster %s not found", netv1alpha1.GroupVersion.String(), clusterID)
			return nil
		} else {
			klog.Errorf("more than one instances of type %s exists for remote cluster %s", netv1alpha1.GroupVersion.String(), clusterID)
			return fmt.Errorf("multiple instances of %s for remote cluster %s", netv1alpha1.GroupVersion.String(), clusterID)
		}
	}
	netConfig := netConfigList.Items[0]
	err = r.Delete(context.Background(), &netConfig)
	if err != nil {
		klog.Errorf("an error occurred while deleting resource %s of type %s: %s", netConfig.Name, netv1alpha1.GroupVersion.String(), err)
		return err
	} else {
		klog.Infof("resource %s of type %s deleted", netConfig.Name, netv1alpha1.GroupVersion.String())
	}
	err = r.deleteTunEndpoint(&netConfig)
	if err != nil {
		klog.Errorf("an error occurred while deleting resource %s of type %s: %s", strings.Join([]string{TunEndpointNamePrefix, netConfig.Spec.ClusterID}, ""), netv1alpha1.GroupVersion.String(), err)
		return err
	} else {
		klog.Infof("resource %s of type %s deleted", strings.Join([]string{TunEndpointNamePrefix, netConfig.Spec.ClusterID}, ""), netv1alpha1.GroupVersion.String())
		return nil
	}

}

func (r *TunnelEndpointCreator) GetNetworkConfig(destinationClusterID string) (*netv1alpha1.NetworkConfig, bool, error) {
	clusterID := destinationClusterID
	networkConfigList := &netv1alpha1.NetworkConfigList{}
	labels := client.MatchingLabels{crdReplicator.DestinationLabel: clusterID}
	err := r.List(context.Background(), networkConfigList, labels)
	if err != nil {
		klog.Errorf("an error occurred while listing resources of type %s: %s", netv1alpha1.GroupVersion, err)
		return nil, false, err
	}
	if len(networkConfigList.Items) != 1 {
		if len(networkConfigList.Items) == 0 {
			return nil, false, nil
		} else {
			klog.Errorf("more than one instances of type %s exists for remote cluster %s", netv1alpha1.GroupVersion.String(), clusterID)
			return nil, false, fmt.Errorf("multiple instances of %s for remote cluster %s", netv1alpha1.GroupVersion.String(), clusterID)
		}
	}
	return &networkConfigList.Items[0], true, nil
}

func (r *TunnelEndpointCreator) processRemoteNetConfig(netConfig *netv1alpha1.NetworkConfig) error {
	//check if the PodCidr of the remote cluster overlaps with any of the subnets on the local cluster
	_, clusterSubnet, err := net.ParseCIDR(netConfig.Spec.PodCIDR)
	if err != nil {
		klog.Errorf("an error occurred while parsing the PodCIDR of resource %s: %s", netConfig.Name, err)
		return err
	}
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	//networkconfigs resources received from remote clusters contains the clusterID of the destination cluster,
	//so in order to take the clusterID of the sender we need to retrieve it from the labels.
	newSubnet, err := r.IPManager.GetNewSubnetPerCluster(clusterSubnet, netConfig.Labels[crdReplicator.RemoteLabelSelector])
	if err != nil {
		klog.Errorf("an error occurred while getting a new subnet for resource %s: %s", netConfig.Name, err)
		return err
	}

	//if they are different, the NAT is needed and a new subnet have been reserved for the peering cluster
	if newSubnet.String() != clusterSubnet.String() {
		if newSubnet.String() != netConfig.Status.PodCIDRNAT {
			//update netConfig status
			netConfig.Status.PodCIDRNAT = newSubnet.String()
			netConfig.Status.NATEnabled = "true"
			err := r.Status().Update(context.Background(), netConfig)
			if err != nil {
				klog.Errorf("an error occurred while updating the status of resource %s: %s", netConfig.Name, err)
				return err
			}
		}
		return nil
	}
	if owner.GetOwnerByKind(&netConfig.OwnerReferences, "ForeignCluster") == nil {
		// if it has no owner of kind ForeignCluster, add it
		own, err := r.getFCOwner(netConfig)
		if err != nil {
			klog.Error(err)
			return err
		}
		if own != nil {
			netConfig.OwnerReferences = append(netConfig.OwnerReferences, *own)
			err = r.Update(context.TODO(), netConfig)
			if err != nil {
				klog.Error(err)
				return err
			}
		}
	}
	if netConfig.Status.PodCIDRNAT != defaultPodCIDRValue {
		//update netConfig status
		netConfig.Status.PodCIDRNAT = defaultPodCIDRValue
		netConfig.Status.NATEnabled = "false"
		err := r.Status().Update(context.Background(), netConfig)
		if err != nil {
			klog.Errorf("an error occurred while updating the status of resource %s: %s", netConfig.Name, err)
			return err
		}
		return nil
	}
	return nil
}

func (r *TunnelEndpointCreator) processLocalNetConfig(netConfig *netv1alpha1.NetworkConfig) error {
	//check if the resource has been processed by the remote cluster
	if netConfig.Status.PodCIDRNAT == "" {
		return nil
	}
	//we get the remote netconfig related to this one
	netConfigList := &netv1alpha1.NetworkConfigList{}
	labels := client.MatchingLabels{crdReplicator.RemoteLabelSelector: netConfig.Spec.ClusterID}
	err := r.List(context.Background(), netConfigList, labels)
	if err != nil {
		klog.Errorf("an error occurred while listing resources: %s", err)
		return err
	}
	if len(netConfigList.Items) != 1 {
		if len(netConfigList.Items) == 0 {
			return nil
		} else {
			klog.Errorf("more than one instances of type %s exists for remote cluster %s", netv1alpha1.GroupVersion.String(), netConfig.Spec.ClusterID)
			return fmt.Errorf("multiple instances of %s for remote cluster %s", netv1alpha1.GroupVersion.String(), netConfig.Spec.ClusterID)
		}
	} else {
		//check if it has been processed by the operator
		if netConfigList.Items[0].Status.NATEnabled == "" {
			return nil
		}
	}
	//at this point we have all the necessary parameters to create the tunnelEndpoint resource
	remoteNetConf := netConfigList.Items[0]
	netParam := networkParam{
		remoteClusterID:  netConfig.Spec.ClusterID,
		remoteGatewayIP:  remoteNetConf.Spec.TunnelPublicIP,
		remotePodCIDR:    remoteNetConf.Spec.PodCIDR,
		remoteNatPodCIDR: remoteNetConf.Status.PodCIDRNAT,
		localNatPodCIDR:  netConfig.Status.PodCIDRNAT,
		localGatewayIP:   netConfig.Spec.TunnelPublicIP,
	}
	fcOwner := owner.GetOwnerByKind(&netConfig.OwnerReferences, "ForeignCluster")
	if err := r.ProcessTunnelEndpoint(netParam, fcOwner); err != nil {
		klog.Errorf("an error occurred while processing the tunnelEndpoint: %s", err)
		return err
	}
	return nil
}

func (r *TunnelEndpointCreator) ProcessTunnelEndpoint(param networkParam, owner *metav1.OwnerReference) error {
	//try to get the tunnelEndpoint, it may not exist
	_, found, err := r.GetTunnelEndpoint(param.remoteClusterID)
	if err != nil {
		klog.Errorf("an error occurred while getting resource tunnelEndpoint for cluster %s: %s", param.remoteClusterID, err)
		return err
	}
	if !found {
		return r.CreateTunnelEndpoint(param, owner)
	} else {
		if err := r.UpdateSpecTunnelEndpoint(param); err != nil {
			return err
		}
		if err := r.UpdateStatusTunnelEndpoint(param); err != nil {
			return err
		}
		return nil
	}
}

func (r *TunnelEndpointCreator) UpdateSpecTunnelEndpoint(param networkParam) error {
	tep := &netv1alpha1.TunnelEndpoint{}
	//here we recover from conflicting resource versions
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toBeUpdated := false
		tep, found, err := r.GetTunnelEndpoint(param.remoteClusterID)
		if err != nil {
			return err
		}
		if !found {
			return apierrors.NewNotFound(netv1alpha1.GroupResource, strings.Join([]string{"tunnelEndpoint for cluster:", param.remoteClusterID}, " "))
		}
		//check if there are fields to be updated
		if tep.Spec.ClusterID != param.remoteClusterID {
			tep.Spec.ClusterID = param.remoteClusterID
			toBeUpdated = true
		}
		if tep.Spec.TunnelPublicIP != param.remoteGatewayIP {
			tep.Spec.TunnelPublicIP = param.remoteGatewayIP
			toBeUpdated = true
		}
		if tep.Spec.PodCIDR != param.remotePodCIDR {
			tep.Spec.PodCIDR = param.remotePodCIDR
			toBeUpdated = true
		}
		if toBeUpdated {
			err = r.Update(context.Background(), tep)
			return err
		}
		return nil
	})
	if retryError != nil {
		klog.Errorf("an error occurred while updating spec of tunnelEndpoint resource %s: %s", tep.Name, retryError)
		return retryError
	}
	return nil
}

func (r *TunnelEndpointCreator) UpdateStatusTunnelEndpoint(param networkParam) error {
	tep := &netv1alpha1.TunnelEndpoint{}

	//here we recover from conflicting resource versions
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toBeUpdated := false
		tep, found, err := r.GetTunnelEndpoint(param.remoteClusterID)
		if err != nil {
			return err
		}
		if !found {
			return apierrors.NewNotFound(netv1alpha1.GroupResource, strings.Join([]string{"tunnelEndpoint for cluster:", param.remoteClusterID}, " "))
		}
		//check if there are fields to be updated
		if tep.Status.LocalRemappedPodCIDR != param.localNatPodCIDR {
			tep.Status.LocalRemappedPodCIDR = param.localNatPodCIDR
			toBeUpdated = true
		}
		if tep.Status.RemoteRemappedPodCIDR != param.remoteNatPodCIDR {
			tep.Status.RemoteRemappedPodCIDR = param.remoteNatPodCIDR
			toBeUpdated = true
		}
		if tep.Status.LocalTunnelPublicIP != param.localGatewayIP {
			tep.Status.LocalTunnelPublicIP = param.localGatewayIP
			toBeUpdated = true
		}
		if tep.Status.RemoteTunnelPublicIP != param.remoteGatewayIP {
			tep.Status.RemoteTunnelPublicIP = param.remoteGatewayIP
			toBeUpdated = true
		}
		if tep.Status.Phase != "Ready" {
			tep.Status.Phase = "Ready"
			toBeUpdated = true
		}
		if toBeUpdated {
			err = r.Status().Update(context.Background(), tep)
			return err
		}
		return nil
	})
	if retryError != nil {
		klog.Errorf("an error occurred while updating status of tunnelEndpoint resource %s: %s", tep.Name, retryError)
		return retryError
	}
	return nil
}

func (r *TunnelEndpointCreator) CreateTunnelEndpoint(param networkParam, owner *metav1.OwnerReference) error {
	//here we create it
	tep := &netv1alpha1.TunnelEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: TunEndpointNamePrefix,
			Labels: map[string]string{
				"clusterID": param.remoteClusterID,
			},
		},
		Spec: netv1alpha1.TunnelEndpointSpec{
			ClusterID:      param.remoteClusterID,
			PodCIDR:        param.remotePodCIDR,
			TunnelPublicIP: param.remoteGatewayIP,
		},
		Status: netv1alpha1.TunnelEndpointStatus{
			Phase:                 "Ready",
			LocalRemappedPodCIDR:  param.localNatPodCIDR,
			RemoteRemappedPodCIDR: param.remoteNatPodCIDR,
			RemoteTunnelPublicIP:  param.remoteGatewayIP,
			LocalTunnelPublicIP:   param.localGatewayIP,
		},
	}
	if owner != nil {
		tep.OwnerReferences = append(tep.OwnerReferences, *owner)
	}
	err := r.Create(context.Background(), tep)
	if err != nil {
		klog.Errorf("an error occurred while creating resource %s of type %s: %s", tep.Name, netv1alpha1.GroupResource, err)
		return err
	} else {
		klog.Infof("resource %s of type %s created", tep.Name, netv1alpha1.GroupResource)
	}
	return nil
}

func (r *TunnelEndpointCreator) ForeignClusterHandlerAdd(obj interface{}) {
	objUnstruct, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting advertisement obj to unstructured object")
		return
	}
	fc := &discoveryv1alpha1.ForeignCluster{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(objUnstruct.Object, fc)
	if err != nil {
		klog.Errorf("an error occurred while converting resource %s of type %s to typed object: %s", objUnstruct.GetName(), objUnstruct.GetKind(), err)
		return
	}
	if fc.Status.Incoming.Joined || fc.Status.Outgoing.Joined {
		_ = r.createNetConfig(fc)
	} else if !fc.Status.Incoming.Joined && !fc.Status.Outgoing.Joined {
		_ = r.deleteNetConfig(fc)
	}
}

func (r *TunnelEndpointCreator) ForeignClusterHandlerUpdate(oldObj interface{}, newObj interface{}) {
	r.ForeignClusterHandlerAdd(newObj)
}

func (r *TunnelEndpointCreator) ForeignClusterHandlerDelete(obj interface{}) {
	objUnstruct, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting advertisement obj to unstructured object")
		return
	}
	fc := &discoveryv1alpha1.ForeignCluster{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(objUnstruct.Object, fc)
	if err != nil {
		klog.Errorf("an error occurred while converting resource %s of type %s to typed object: %s", objUnstruct.GetName(), objUnstruct.GetKind(), err)
		return
	}
	_ = r.deleteNetConfig(fc)
}

func (r *TunnelEndpointCreator) GetTunnelEndpoint(destinationClusterID string) (*netv1alpha1.TunnelEndpoint, bool, error) {
	clusterID := destinationClusterID
	tunEndpointList := &netv1alpha1.TunnelEndpointList{}
	labels := client.MatchingLabels{"clusterID": clusterID}
	err := r.List(context.Background(), tunEndpointList, labels)
	if err != nil {
		klog.Errorf("an error occurred while listing resources: %s", err)
		return nil, false, err
	}
	if len(tunEndpointList.Items) != 1 {
		if len(tunEndpointList.Items) == 0 {
			return nil, false, nil
		} else {
			klog.Errorf("more than one instances of type %s exists for remote cluster %s", netv1alpha1.GroupVersion.String(), clusterID)
			return nil, false, fmt.Errorf("multiple instances of %s for remote cluster %s", netv1alpha1.GroupVersion.String(), clusterID)
		}
	}
	return &tunEndpointList.Items[0], true, nil
}

func (r *TunnelEndpointCreator) deleteTunEndpoint(netConfig *netv1alpha1.NetworkConfig) error {
	ctx := context.Background()
	clusterID := netConfig.Spec.ClusterID
	tep, found, err := r.GetTunnelEndpoint(clusterID)
	if err != nil {
		return err
	}
	if !found {
		klog.Infof("tunnelendpoint resource for cluster %s not found", clusterID)
		return nil
	}
	err = r.Delete(ctx, tep)
	if err != nil {
		return fmt.Errorf("unable to delete endpoint %s in namespace %s : %v", tep.Name, tep.Namespace, err)
	} else {
		klog.Infof("resource %s of type %s for remote cluster %s has been removed", tep.Name, netv1alpha1.GroupVersion.String(), clusterID)
		return nil
	}
}

func (r *TunnelEndpointCreator) getFCOwner(netConfig *netv1alpha1.NetworkConfig) (*metav1.OwnerReference, error) {
	dynFC := r.DynClient.Resource(schema.GroupVersionResource{
		Group:    discoveryv1alpha1.GroupVersion.Group,
		Version:  discoveryv1alpha1.GroupVersion.Version,
		Resource: "foreignclusters",
	})
	list, err := dynFC.List(context.TODO(), metav1.ListOptions{
		LabelSelector: strings.Join([]string{"cluster-id", netConfig.Spec.ClusterID}, "="),
	})
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	fc := list.Items[0]
	return &metav1.OwnerReference{
		APIVersion: fc.GetAPIVersion(),
		Kind:       fc.GetKind(),
		Name:       fc.GetName(),
		UID:        fc.GetUID(),
		Controller: pointer.BoolPtr(true),
	}, nil
}
