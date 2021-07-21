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

package tunnelEndpointCreator

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/utils"
)

const (
	tunEndpointNamePrefix = "tun-endpoint-"
	netConfigNamePrefix   = "net-config-"
)

var (
	// ResyncPeriod for watchers.
	ResyncPeriod = 30 * time.Second

	result = ctrl.Result{
		Requeue:      false,
		RequeueAfter: 5 * time.Second,
	}
)

type networkParam struct {
	remoteClusterID       string
	remoteEndpointIP      string
	remotePodCIDR         string
	remoteNatPodCIDR      string
	remoteExternalCIDR    string
	remoteNatExternalCIDR string
	localEndpointIP       string
	localNatPodCIDR       string
	localPodCIDR          string
	localExternalCIDR     string
	localNatExternalCIDR  string
	backendType           string
	backendConfig         map[string]string
}

// TunnelEndpointCreator manages the most of liqo networking.
type TunnelEndpointCreator struct {
	client.Client
	ClientSet                  *k8s.Clientset
	Scheme                     *runtime.Scheme
	Manager                    ctrl.Manager
	DynClient                  dynamic.Interface
	EndpointIP                 string
	EndpointPort               string
	PodCIDR                    string
	ServiceCIDR                string
	ExternalCIDR               string
	ReservedSubnets            []string
	AdditionalPools            []string
	IPManager                  liqonetIpam.Ipam
	Mutex                      sync.Mutex
	WaitConfig                 *sync.WaitGroup
	IpamConfigured             bool
	wgPubKey                   string
	IsConfigured               bool
	Configured                 chan bool
	ForeignClusterStartWatcher chan bool
	ForeignClusterStopWatcher  chan struct{}
	secretClusterStopChan      chan struct{}
	SecretStopWatcher          chan struct{}
	RunningWatchers            bool
	Namespace                  string
	wgConfigured               bool
	svcConfigured              bool
	cfgConfigured              bool
	RetryTimeout               time.Duration
}

// rbac for the net.liqo.io api
// cluster-role
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=net.liqo.io,resources=ipamstorages,verbs=get;list;create;update;patch
// +kubebuilder:rbac:groups=net.liqo.io,resources=natmappings,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=services,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=pods,verbs=get;list;watch

// Reconciler method.
func (tec *TunnelEndpointCreator) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !tec.IsConfigured {
		klog.Infof("the operator is waiting to be configured")
		tec.WaitConfig.Wait()
		klog.Infof("operator configured")
		tec.IsConfigured = true
	}
	tunnelEndpointCreatorFinalizer := "tunnelEndpointCreator-Finalizer.liqonet.liqo.io"
	// get networkConfig
	var netConfig netv1alpha1.NetworkConfig
	if err := tec.Get(ctx, req.NamespacedName, &netConfig); apierrors.IsNotFound(err) {
		// reconcile was triggered by a delete request
		return ctrl.Result{}, client.IgnoreNotFound(err)
	} else if err != nil {
		klog.Errorf("an error occurred while getting resource %s: %s", req.NamespacedName, err)
		return result, err
	}
	// examine DeletionTimestamp to determine if object is under deletion
	if netConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&netConfig, tunnelEndpointCreatorFinalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			controllerutil.AddFinalizer(&netConfig, tunnelEndpointCreatorFinalizer)
			if err := tec.Update(ctx, &netConfig); err != nil {
				// while updating we check if the a resource version conflict happened
				// which means the version of the object we have is outdated.
				// a solution could be to return an error and requeue the object for later process
				// but if the object has been changed by another instance of the controller running in
				// another host it already has been put in the working queue so decide to forget the
				// current version and process the next item in the queue assured that we handle the object later
				if apierrors.IsConflict(err) {
					return ctrl.Result{}, nil
				}
				klog.Errorf("an error occurred while setting finalizer for resource %s: %s", req.NamespacedName, err)
				return result, err
			}
			return result, nil
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(&netConfig, tunnelEndpointCreatorFinalizer) {
			// Remove IPAM configuration per cluster
			if err := tec.IPManager.RemoveClusterConfig(netConfig.Spec.ClusterID); err != nil {
				klog.Errorf("cannot delete local subnets assigned to cluster %s: %s", netConfig.Spec.ClusterID, err.Error())
				return result, err
			}
			// Remove TunnelEndpoint resource relative to this NetworkConfig
			if err := tec.deleteTunEndpoint(&netConfig); err != nil {
				klog.Errorf("an error occurred while deleting tunnel endpoint related to %s: %s", netConfig.Name, err)
				return result, err
			}
			// Remove the finalizer and update resource.
			controllerutil.RemoveFinalizer(&netConfig, tunnelEndpointCreatorFinalizer)
			if err := tec.Update(ctx, &netConfig); err != nil {
				klog.Errorf("an error occurred while removing finalizer from resource %s: %s", req.NamespacedName, err)
				return result, err
			}
		}
		return result, nil
	}

	// check if the netconfig is local or remote
	labels := netConfig.GetLabels()
	if val, ok := labels[crdreplicator.LocalLabelSelector]; ok && val == "true" {
		return result, tec.processLocalNetConfig(&netConfig)
	}
	if _, ok := labels[crdreplicator.RemoteLabelSelector]; ok {
		return result, tec.processRemoteNetConfig(&netConfig)
	}
	return result, nil
}

// SetupWithManager informs the manager that the tunnelEndpointCreator will deal with networkconfigs.
func (tec *TunnelEndpointCreator) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1alpha1.NetworkConfig{}).
		Complete(tec)
}

// SetupSignalHandlerForTunEndCreator registers for SIGTERM, SIGINT, SIGKILL. A stop channel is returned
// which is closed on one of these signals.
func (tec *TunnelEndpointCreator) SetupSignalHandlerForTunEndCreator() context.Context {
	klog.Infof("starting signal handler for tunnelEndpointCreator-operator")
	ctx, done := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, liqonetutils.ShutdownSignals...)
	go func(r *TunnelEndpointCreator) {
		sig := <-c
		klog.Infof("received signal: %s", sig.String())
		// Stop IPAM.
		tec.IPManager.Terminate()
		// closing shared informers
		close(r.ForeignClusterStopWatcher)
		done()
	}(tec)
	return ctx
}

// Watcher for resources.
func (tec *TunnelEndpointCreator) Watcher(sharedDynFactory dynamicinformer.DynamicSharedInformerFactory,
	resourceType schema.GroupVersionResource,
	handlerFuncs cache.ResourceEventHandlerFuncs,
	stopCh chan struct{}) {
	dynInformer := sharedDynFactory.ForResource(resourceType)
	klog.Infof("starting watcher for %s", resourceType.String())
	// adding handlers to the informer
	dynInformer.Informer().AddEventHandler(handlerFuncs)
	dynInformer.Informer().Run(stopCh)
}

func (tec *TunnelEndpointCreator) createNetConfig(fc *discoveryv1alpha1.ForeignCluster) error {
	clusterID := fc.Spec.ClusterIdentity.ClusterID
	netConfig := netv1alpha1.NetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: netConfigNamePrefix,
			Namespace:    fc.Status.TenantNamespace.Local,
			Labels: map[string]string{
				crdreplicator.LocalLabelSelector: "true",
				crdreplicator.DestinationLabel:   clusterID,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: fmt.Sprintf("%s/%s", discoveryv1alpha1.GroupVersion.Group, discoveryv1alpha1.GroupVersion.Version),
					Kind:       "ForeignCluster",
					Name:       fc.Name,
					UID:        fc.UID,
					Controller: pointer.BoolPtr(true),
				},
			},
		},
		Spec: netv1alpha1.NetworkConfigSpec{
			ClusterID:    clusterID,
			PodCIDR:      tec.PodCIDR,
			ExternalCIDR: tec.ExternalCIDR,
			EndpointIP:   tec.EndpointIP,
			BackendType:  wireguard.DriverName,
			BackendConfig: map[string]string{
				wireguard.PublicKey:     tec.wgPubKey,
				wireguard.ListeningPort: tec.EndpointPort,
			},
		},
		Status: netv1alpha1.NetworkConfigStatus{},
	}
	// check if the resource for the remote cluster already exists
	_, exists, err := tec.GetNetworkConfig(clusterID, fc.Status.TenantNamespace.Local)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	err = tec.Create(context.TODO(), &netConfig)
	if err != nil {
		klog.Errorf("an error occurred while creating resource %s of type %s: %s",
			netConfig.Name, netv1alpha1.GroupVersion.String(), err)
		return err
	}
	klog.Infof("resource %s of type %s created", netConfig.Name, netv1alpha1.GroupVersion.String())
	return nil
}

func (tec *TunnelEndpointCreator) deleteNetConfig(fc *discoveryv1alpha1.ForeignCluster) error {
	clusterID := fc.Spec.ClusterIdentity.ClusterID
	netConfigList := &netv1alpha1.NetworkConfigList{}
	labels := client.MatchingLabels{crdreplicator.DestinationLabel: clusterID}
	err := tec.List(context.Background(), netConfigList, labels, client.InNamespace(fc.Status.TenantNamespace.Local))
	if err != nil {
		klog.Errorf("an error occurred while listing resources: %s", err)
		return err
	}
	if len(netConfigList.Items) != 1 {
		if len(netConfigList.Items) == 0 {
			klog.V(4).Infof("nothing to remove: a resource of type %s for remote cluster %s not found",
				netv1alpha1.GroupVersion.String(), clusterID)
			return nil
		}
		klog.Errorf("more than one instances of type %s exists for remote cluster %s",
			netv1alpha1.GroupVersion.String(), clusterID)
		return fmt.Errorf("multiple instances of %s for remote cluster %s", netv1alpha1.GroupVersion.String(), clusterID)
	}
	netConfig := netConfigList.Items[0]
	err = tec.Delete(context.Background(), &netConfig)
	if err != nil {
		klog.Errorf("an error occurred while deleting resource %s of type %s: %s",
			netConfig.Name, netv1alpha1.GroupVersion.String(), err)
		return err
	}
	klog.Infof("resource %s of type %s deleted", netConfig.Name, netv1alpha1.GroupVersion.String())
	err = tec.deleteTunEndpoint(&netConfig)
	if err != nil {
		klog.Errorf("an error occurred while deleting resource %s of type %s: %s",
			strings.Join([]string{tunEndpointNamePrefix, netConfig.Spec.ClusterID}, ""), netv1alpha1.GroupVersion.String(), err)
		return err
	}
	klog.Infof("resource %s of type %s deleted",
		strings.Join([]string{tunEndpointNamePrefix, netConfig.Spec.ClusterID}, ""),
		netv1alpha1.GroupVersion.String())
	return nil
}

// GetNetworkConfig returns the network config for a specific cluster.
func (tec *TunnelEndpointCreator) GetNetworkConfig(destinationClusterID, namespace string) (
	*netv1alpha1.NetworkConfig,
	bool,
	error) {
	clusterID := destinationClusterID
	networkConfigList := &netv1alpha1.NetworkConfigList{}
	labels := client.MatchingLabels{crdreplicator.DestinationLabel: clusterID}
	err := tec.List(context.Background(), networkConfigList, labels, client.InNamespace(namespace))
	if err != nil {
		klog.Errorf("an error occurred while listing resources of type %s: %s", netv1alpha1.GroupVersion, err)
		return nil, false, err
	}
	if len(networkConfigList.Items) != 1 {
		if len(networkConfigList.Items) == 0 {
			return nil, false, nil
		}
		klog.Errorf("more than one instances of type %s exists for remote cluster %s",
			netv1alpha1.GroupVersion.String(), clusterID)
		return nil, false, fmt.Errorf("multiple instances of %s for remote cluster %s",
			netv1alpha1.GroupVersion.String(), clusterID)
	}
	return &networkConfigList.Items[0], true, nil
}

func (tec *TunnelEndpointCreator) processRemoteNetConfig(netConfig *netv1alpha1.NetworkConfig) error {
	var toBeUpdated bool
	// Networkconfigs resource have a Spec.ClusterID field that contains
	// the clusterid of the destination cluster(the local cluster in this case)
	// In order to take the ClusterID of the sender we need to retrieve it from the labels.
	podCIDR, externalCIDR, err := tec.IPManager.GetSubnetsPerCluster(netConfig.Spec.PodCIDR,
		netConfig.Spec.ExternalCIDR, netConfig.Labels[crdreplicator.RemoteLabelSelector])
	if err != nil {
		klog.Errorf("an error occurred while getting a new subnet for resource %s: %s", netConfig.Name, err)
		return err
	}

	// Following inner if statements guarrante that we update the resource only if we need to
	if podCIDR != netConfig.Spec.PodCIDR {
		// Local cluster has remapped the PodCIDR of the remote cluster because of conflicts
		if podCIDR != netConfig.Status.PodCIDRNAT {
			toBeUpdated = true
		}
	} else {
		// Local cluster has not remapped the PodCIDR
		if netConfig.Status.PodCIDRNAT != liqoconst.DefaultCIDRValue {
			toBeUpdated = true
			podCIDR = liqoconst.DefaultCIDRValue
		}
	}
	if externalCIDR != netConfig.Spec.ExternalCIDR {
		// Local cluster has remapped the ExternalCIDR of the remote cluster because of conflicts
		if externalCIDR != netConfig.Status.ExternalCIDRNAT {
			toBeUpdated = true
		}
	} else {
		// Local cluster has not remapped the ExternalCIDR
		if netConfig.Status.ExternalCIDRNAT != liqoconst.DefaultCIDRValue {
			toBeUpdated = true
			externalCIDR = liqoconst.DefaultCIDRValue
		}
	}
	if utils.GetOwnerByKind(&netConfig.OwnerReferences, "ForeignCluster") == nil {
		// if it has no owner of kind ForeignCluster, add it
		own, err := tec.getFCOwner(netConfig)
		if err != nil {
			klog.Error(err)
			return err
		}
		if own != nil {
			netConfig.OwnerReferences = append(netConfig.OwnerReferences, *own)
			err = tec.Update(context.TODO(), netConfig)
			if err != nil {
				klog.Error(err)
				return err
			}
			return nil
		}
	}

	if toBeUpdated {
		netConfig.Status.Processed = true
		netConfig.Status.PodCIDRNAT = podCIDR
		netConfig.Status.ExternalCIDRNAT = externalCIDR
		err := tec.Status().Update(context.Background(), netConfig)
		if err != nil {
			klog.Errorf("an error occurred while updating the status of resource %s: %s", netConfig.Name, err)
			return err
		}
	}
	return nil
}

func (tec *TunnelEndpointCreator) processLocalNetConfig(netConfig *netv1alpha1.NetworkConfig) error {
	// first check that this is the only resource for the remote cluster
	netConfigList := &netv1alpha1.NetworkConfigList{}
	labels := client.MatchingLabels{crdreplicator.DestinationLabel: netConfig.Labels[crdreplicator.DestinationLabel]}
	err := tec.List(context.Background(), netConfigList, labels, client.InNamespace(netConfig.GetNamespace()))
	if err != nil {
		klog.Errorf("an error occurred while listing resources: %s", err)
		return err
	}
	if len(netConfigList.Items) != 1 {
		if len(netConfigList.Items) == 0 {
			return nil
		}
		klog.Warningf("more than one instances of type %s exists for remote cluster %s, deleting them",
			netv1alpha1.GroupVersion.String(), netConfig.Spec.ClusterID)
		for i := range netConfigList.Items {
			klog.Infof("deleting resource %s with GVR %s, because it is duplicated",
				netConfigList.Items[i].Name, netConfigList.Items[i].GroupVersionKind().String())
			if err := tec.Delete(context.Background(), &netConfigList.Items[i]); err != nil {
				klog.Errorf("an error occurred while deleting resource %s: %v", netConfigList.Items[i].Name, err)
				return err
			}
		}
		return nil
	}
	// check if the resource has been processed by the remote cluster
	if !netConfig.Status.Processed {
		return nil
	}
	// we get the remote netconfig related to this one
	netConfigList = &netv1alpha1.NetworkConfigList{}
	labels = client.MatchingLabels{crdreplicator.RemoteLabelSelector: netConfig.Spec.ClusterID}
	err = tec.List(context.Background(), netConfigList, labels)
	if err != nil {
		klog.Errorf("an error occurred while listing resources: %s", err)
		return err
	}
	if len(netConfigList.Items) != 1 {
		if len(netConfigList.Items) == 0 {
			return nil
		}
		klog.Errorf("more than one instances of type %s exists for remote cluster %s",
			netv1alpha1.GroupVersion.String(), netConfig.Spec.ClusterID)
		return fmt.Errorf("multiple instances of %s for remote cluster %s",
			netv1alpha1.GroupVersion.String(), netConfig.Spec.ClusterID)
	}
	if !netConfigList.Items[0].Status.Processed {
		return nil
	}

	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Store subnets used in remote cluster
		if err := tec.IPManager.AddLocalSubnetsPerCluster(netConfig.Status.PodCIDRNAT,
			netConfig.Status.ExternalCIDRNAT,
			netConfig.Spec.ClusterID); err != nil {
			return err
		}
		return nil
	})
	if retryError != nil {
		klog.Error(retryError)
		return retryError
	}
	// at this point we have all the necessary parameters to create the tunnelEndpoint resource
	remoteNetConf := netConfigList.Items[0]
	netParam := networkParam{
		remoteClusterID:       netConfig.Spec.ClusterID,
		remoteEndpointIP:      remoteNetConf.Spec.EndpointIP,
		remotePodCIDR:         remoteNetConf.Spec.PodCIDR,
		remoteNatPodCIDR:      remoteNetConf.Status.PodCIDRNAT,
		remoteExternalCIDR:    remoteNetConf.Spec.ExternalCIDR,
		remoteNatExternalCIDR: remoteNetConf.Status.ExternalCIDRNAT,
		localNatPodCIDR:       netConfig.Status.PodCIDRNAT,
		localEndpointIP:       netConfig.Spec.EndpointIP,
		localPodCIDR:          netConfig.Spec.PodCIDR,
		localExternalCIDR:     netConfig.Spec.ExternalCIDR,
		localNatExternalCIDR:  netConfig.Status.ExternalCIDRNAT,
		backendType:           remoteNetConf.Spec.BackendType,
		backendConfig:         remoteNetConf.Spec.BackendConfig,
	}
	fcOwner := utils.GetOwnerByKind(&netConfig.OwnerReferences, "ForeignCluster")
	if err := tec.processTunnelEndpoint(&netParam, fcOwner, netConfig.GetNamespace()); err != nil {
		klog.Errorf("an error occurred while processing the tunnelEndpoint: %s", err)
		return err
	}
	return nil
}

func (tec *TunnelEndpointCreator) processTunnelEndpoint(param *networkParam, ownerRef *metav1.OwnerReference, namespace string) error {
	// try to get the tunnelEndpoint, it may not exist
	_, found, err := tec.GetTunnelEndpoint(param.remoteClusterID, namespace)
	if err != nil {
		klog.Errorf("an error occurred while getting resource tunnelEndpoint for cluster %s: %s", param.remoteClusterID, err)
		return err
	}
	if !found {
		return tec.createTunnelEndpoint(param, ownerRef, namespace)
	}
	if err := tec.updateSpecTunnelEndpoint(param, namespace); err != nil {
		return err
	}
	if err := tec.updateStatusTunnelEndpoint(param, namespace); err != nil {
		return err
	}
	return nil
}

func (tec *TunnelEndpointCreator) updateSpecTunnelEndpoint(param *networkParam, namespace string) error {
	var tep *netv1alpha1.TunnelEndpoint
	var found bool
	var err error
	// here we recover from conflicting resource versions
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toBeUpdated := false
		tep, found, err = tec.GetTunnelEndpoint(param.remoteClusterID, namespace)
		if err != nil {
			return err
		}
		if !found {
			return apierrors.NewNotFound(netv1alpha1.TunnelEndpointGroupResource,
				strings.Join([]string{"tunnelEndpoint for cluster:", param.remoteClusterID}, " "))
		}
		// check if there are fields to be updated
		if tep.Spec.ClusterID != param.remoteClusterID {
			tep.Spec.ClusterID = param.remoteClusterID
			toBeUpdated = true
		}
		if tep.Spec.EndpointIP != param.remoteEndpointIP {
			tep.Spec.EndpointIP = param.remoteEndpointIP
			toBeUpdated = true
		}
		if tep.Spec.PodCIDR != param.remotePodCIDR {
			tep.Spec.PodCIDR = param.remotePodCIDR
			toBeUpdated = true
		}
		if tep.Spec.ExternalCIDR != param.remoteExternalCIDR {
			tep.Spec.ExternalCIDR = param.remoteExternalCIDR
			toBeUpdated = true
		}
		if !reflect.DeepEqual(tep.Spec.BackendConfig, param.backendConfig) {
			tep.Spec.BackendConfig = param.backendConfig
			toBeUpdated = true
		}
		if toBeUpdated {
			err = tec.Update(context.Background(), tep)
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

func (tec *TunnelEndpointCreator) updateStatusTunnelEndpoint(param *networkParam, namespace string) error {
	var tep *netv1alpha1.TunnelEndpoint
	var found bool
	var err error

	// here we recover from conflicting resource versions
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toBeUpdated := false
		tep, found, err = tec.GetTunnelEndpoint(param.remoteClusterID, namespace)
		if err != nil {
			return err
		}
		if !found {
			return apierrors.NewNotFound(netv1alpha1.TunnelEndpointGroupResource,
				strings.Join([]string{"tunnelEndpoint for cluster:", param.remoteClusterID}, " "))
		}
		// check if there are fields to be updated
		if tep.Status.LocalNATPodCIDR != param.localNatPodCIDR {
			tep.Status.LocalNATPodCIDR = param.localNatPodCIDR
			toBeUpdated = true
		}
		if tep.Status.RemoteNATPodCIDR != param.remoteNatPodCIDR {
			tep.Status.RemoteNATPodCIDR = param.remoteNatPodCIDR
			toBeUpdated = true
		}
		if tep.Status.LocalEndpointIP != param.localEndpointIP {
			tep.Status.LocalEndpointIP = param.localEndpointIP
			toBeUpdated = true
		}
		if tep.Status.RemoteEndpointIP != param.remoteEndpointIP {
			tep.Status.RemoteEndpointIP = param.remoteEndpointIP
			toBeUpdated = true
		}
		if tep.Status.LocalPodCIDR != param.localPodCIDR {
			tep.Status.LocalPodCIDR = param.localPodCIDR
			toBeUpdated = true
		}
		if tep.Status.LocalExternalCIDR != param.localExternalCIDR {
			tep.Status.LocalExternalCIDR = param.localExternalCIDR
			toBeUpdated = true
		}
		if tep.Status.LocalNATExternalCIDR != param.localNatExternalCIDR {
			tep.Status.LocalNATExternalCIDR = param.localNatExternalCIDR
			toBeUpdated = true
		}
		if tep.Status.RemoteNATExternalCIDR != param.remoteNatExternalCIDR {
			tep.Status.RemoteNATExternalCIDR = param.remoteNatExternalCIDR
			toBeUpdated = true
		}
		if tep.Status.Phase != liqoconst.TepReady {
			tep.Status.Phase = liqoconst.TepReady
			toBeUpdated = true
		}
		if toBeUpdated {
			err = tec.Status().Update(context.Background(), tep)
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

func (tec *TunnelEndpointCreator) createTunnelEndpoint(param *networkParam, ownerRef *metav1.OwnerReference, namespace string) error {
	// here we create it
	tep := &netv1alpha1.TunnelEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: tunEndpointNamePrefix,
			Namespace:    namespace,
			Labels: map[string]string{
				liqoconst.ClusterIDLabelName: param.remoteClusterID,
			},
		},
		Spec: netv1alpha1.TunnelEndpointSpec{
			ClusterID:     param.remoteClusterID,
			PodCIDR:       param.remotePodCIDR,
			ExternalCIDR:  param.remoteExternalCIDR,
			EndpointIP:    param.remoteEndpointIP,
			BackendType:   param.backendType,
			BackendConfig: param.backendConfig,
		},
		Status: netv1alpha1.TunnelEndpointStatus{
			Phase:                 liqoconst.TepReady,
			LocalPodCIDR:          param.localPodCIDR,
			LocalNATPodCIDR:       param.localNatPodCIDR,
			LocalExternalCIDR:     param.localExternalCIDR,
			LocalNATExternalCIDR:  param.localNatExternalCIDR,
			RemoteNATExternalCIDR: param.remoteNatExternalCIDR,
			RemoteNATPodCIDR:      param.remoteNatPodCIDR,
			RemoteEndpointIP:      param.remoteEndpointIP,
			LocalEndpointIP:       param.localEndpointIP,
		},
	}
	if ownerRef != nil {
		tep.OwnerReferences = append(tep.OwnerReferences, *ownerRef)
	}
	err := tec.Create(context.Background(), tep)
	if err != nil {
		klog.Errorf("an error occurred while creating resource %s of type %s: %s",
			tep.Name, netv1alpha1.TunnelEndpointGroupResource, err)
		return err
	}
	klog.Infof("resource %s of type %s created", tep.Name, netv1alpha1.TunnelEndpointGroupResource)

	return nil
}

// GetTunnelEndpoint retrieves the tunnelEndpoint resource related to a cluster.
func (tec *TunnelEndpointCreator) GetTunnelEndpoint(destinationClusterID, namespace string) (
	*netv1alpha1.TunnelEndpoint,
	bool,
	error) {
	clusterID := destinationClusterID
	tunEndpointList := &netv1alpha1.TunnelEndpointList{}
	labels := client.MatchingLabels{liqoconst.ClusterIDLabelName: clusterID}
	err := tec.List(context.Background(), tunEndpointList, labels, client.InNamespace(namespace))
	if err != nil {
		klog.Errorf("an error occurred while listing resources: %s", err)
		return nil, false, err
	}
	if len(tunEndpointList.Items) != 1 {
		if len(tunEndpointList.Items) == 0 {
			return nil, false, nil
		}
		klog.Errorf("more than one instances of type %s exists for remote cluster %s",
			netv1alpha1.GroupVersion.String(), clusterID)
		return nil, false, fmt.Errorf("multiple instances of %s for remote cluster %s",
			netv1alpha1.GroupVersion.String(), clusterID)
	}
	return &tunEndpointList.Items[0], true, nil
}

func (tec *TunnelEndpointCreator) deleteTunEndpoint(netConfig *netv1alpha1.NetworkConfig) error {
	ctx := context.Background()
	clusterID := netConfig.Spec.ClusterID
	tep, found, err := tec.GetTunnelEndpoint(clusterID, netConfig.GetNamespace())
	if err != nil {
		return err
	}
	if !found {
		klog.Infof("tunnelendpoint resource for cluster %s not found", clusterID)
		return nil
	}
	err = tec.Delete(ctx, tep)
	if err != nil {
		return fmt.Errorf("unable to delete endpoint %s in namespace %s : %w", tep.Name, tep.Namespace, err)
	}
	klog.Infof("resource %s of type %s for remote cluster %s has been removed",
		tep.Name, netv1alpha1.GroupVersion.String(), clusterID)
	return nil
}

func (tec *TunnelEndpointCreator) getFCOwner(netConfig *netv1alpha1.NetworkConfig) (*metav1.OwnerReference, error) {
	dynFC := tec.DynClient.Resource(schema.GroupVersionResource{
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
