// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tunnelendpointcreator

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/internal/liqonet/network-manager/netcfgcreator"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/utils"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

type networkParam struct {
	remoteCluster         discoveryv1alpha1.ClusterIdentity
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
	Scheme    *runtime.Scheme
	IPManager liqonetIpam.Ipam
}

// rbac for the net.liqo.io api
// cluster-role
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=net.liqo.io,resources=ipamstorages,verbs=get;list;create;update;patch
// +kubebuilder:rbac:groups=net.liqo.io,resources=natmappings,verbs=get;list;create;update;patch;delete;watch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status;foreignclusters/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles the state of NetworkConfig resources.
func (tec *TunnelEndpointCreator) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	tunnelEndpointCreatorFinalizer := strings.Join([]string{"tunnelendpointcreator", liqoconst.FinalizersSuffix}, ".")

	klog.V(4).Infof("Reconciling NetworkConfig %q", req)
	tracer := trace.New("Reconcile", trace.Field{Key: "NetworkConfig", Value: req.Name})
	ctx = trace.ContextWithTrace(ctx, tracer)
	defer tracer.LogIfLong(traceutils.LongThreshold())

	// get networkConfig
	var netConfig netv1alpha1.NetworkConfig
	if err := tec.Get(ctx, req.NamespacedName, &netConfig); apierrors.IsNotFound(err) {
		// reconcile was triggered by a delete request
		return ctrl.Result{}, client.IgnoreNotFound(err)
	} else if err != nil {
		klog.Errorf("an error occurred while getting resource %s: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
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
				return ctrl.Result{}, err
			}
			tracer.Step("Finalizer configuration")
			klog.V(4).Infof("Successfully added finalizer to NetworkConfig %q", req)
			return ctrl.Result{}, nil
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(&netConfig, tunnelEndpointCreatorFinalizer) {
			// Remove IPAM configuration per cluster
			if err := tec.IPManager.RemoveClusterConfig(netConfig.Spec.RemoteCluster.ClusterID); err != nil {
				klog.Errorf("cannot delete local subnets assigned to cluster %s: %s", netConfig.Spec.RemoteCluster, err.Error())
				return ctrl.Result{}, err
			}
			// Remove TunnelEndpoint resource relative to this NetworkConfig
			if err := tec.deleteTunEndpoint(ctx, &netConfig); err != nil {
				klog.Errorf("an error occurred while deleting tunnel endpoint related to %s: %s", netConfig.Name, err)
				return ctrl.Result{}, err
			}
			// Remove the finalizer and update resource.
			controllerutil.RemoveFinalizer(&netConfig, tunnelEndpointCreatorFinalizer)
			if err := tec.Update(ctx, &netConfig); err != nil {
				klog.Errorf("an error occurred while removing finalizer from resource %s: %s", req.NamespacedName, err)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if the netconfig is local or remote, and retrieve the cluster ID of the remote cluster
	var clusterID string
	if val, ok := netConfig.GetLabels()[liqoconst.ReplicationRequestedLabel]; ok && val == "true" {
		// This is a local NetworkConfig
		clusterID = netConfig.Spec.RemoteCluster.ClusterID
	} else if cid, ok := netConfig.GetLabels()[liqoconst.ReplicationOriginLabel]; ok {
		// This is a remote NetworkConfig
		// the cluster ID in the spec is the one of the destination cluster (the local cluster in this case)
		// In order to take the ClusterID of the sender we need to retrieve it from the labels.
		clusterID = cid
	} else {
		klog.Warning("NetworkConfig %q is invalid, as neither local nor remote", klog.KObj(&netConfig))
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, tec.processNetworkConfig(ctx, clusterID, netConfig.Namespace)
}

// SetupWithManager informs the manager that the tunnelEndpointCreator will deal with networkconfigs.
func (tec *TunnelEndpointCreator) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1alpha1.NetworkConfig{}).
		Watches(&source.Kind{Type: &netv1alpha1.TunnelEndpoint{}},
			&handler.EnqueueRequestForOwner{OwnerType: &netv1alpha1.NetworkConfig{}, IsController: false}).
		Complete(tec)
}

// SetupSignalHandlerForTunEndCreator registers for SIGTERM, SIGINT, SIGKILL. A stop channel is returned
// which is closed on one of these signals.
func (tec *TunnelEndpointCreator) SetupSignalHandlerForTunEndCreator() context.Context {
	klog.Infof("starting signal handler for tunnelEndpointCreator-operator")
	ctx, done := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, liqonetutils.ShutdownSignals...)
	go func() {
		sig := <-c
		klog.Infof("received signal: %s", sig.String())
		// Stop IPAM.
		tec.IPManager.Terminate()
		done()
	}()
	return ctx
}

func (tec *TunnelEndpointCreator) processNetworkConfig(ctx context.Context, clusterID, namespace string) error {
	tracer := trace.FromContext(ctx)
	klog.V(4).Infof("Processing NetworkConfigs for cluster ID %v", clusterID)

	// Get the NetworkConfig created by the remote cluster.
	// In case a duplicate is found, it is not immediately deleted, since it will be
	// recollected by the origin cluster and eventually propagated here.
	remote, err := netcfgcreator.GetRemoteNetworkConfig(ctx, tec.Client, clusterID, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("No remote NetworkConfig for cluster %v found yet", clusterID)
			return nil
		}

		klog.Errorf("Failed to retrieve remote NetworkConfig for cluster %v: %v", clusterID, err)
		return err
	}

	// Process the remote NetworkConfig, and enforce its meta (i.e. owner reference) and status
	klog.V(4).Infof("Retrieved remote NetworkConfig %q for cluster %v", klog.KObj(remote), clusterID)
	tracer.Step("Remote NetworkConfig retrieval")
	if err := tec.enforceRemoteNetConfigMeta(ctx, remote); err != nil {
		return err
	}
	tracer.Step("Remote NetworkConfig meta enforcement")
	if err := tec.enforceRemoteNetConfigStatus(ctx, remote); err != nil {
		return err
	}
	tracer.Step("Remote NetworkConfig status enforcement")

	// Get the NetworkConfig created by the local cluster.
	local, err := netcfgcreator.GetLocalNetworkConfig(ctx, tec.Client, nil, clusterID, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("No local NetworkConfig for cluster %v found yet", clusterID)
			return nil
		}

		klog.Error("failed to retrieve local NetworkConfig for cluster %v: %v", clusterID, err)
		return err
	}

	// Check if the resource has been processed by the remote cluster
	klog.V(4).Infof("Retrieved local NetworkConfig %q for cluster %v", klog.KObj(local), clusterID)
	tracer.Step("Local NetworkConfig retrieval")
	if !local.Status.Processed {
		klog.V(4).Infof("Local NetworkConfig %q has not yet been processed by the remote cluster %v", klog.KObj(local), clusterID)
		return nil
	}

	if err := tec.IPManager.AddLocalSubnetsPerCluster(local.Status.PodCIDRNAT, local.Status.ExternalCIDRNAT, clusterID); err != nil {
		klog.Errorf("Failed to add local subnets to IPAM for cluster %s: %v", local.Spec.RemoteCluster, err)
		return err
	}
	tracer.Step("IPAM configuration")

	// If we reached this point, then it is possible to enforce the TunnelEndpoint resource
	return tec.enforceTunnelEndpoint(ctx, local, remote)
}

func (tec *TunnelEndpointCreator) enforceRemoteNetConfigMeta(ctx context.Context, netcfg *netv1alpha1.NetworkConfig) error {
	clusterID := netcfg.Labels[liqoconst.ReplicationOriginLabel]

	// Add the ForeignCluster owner reference if not already present, to allow its operator
	// to be triggered and reconcile its status when this NetworkConfig changes.
	if utils.GetOwnerByKind(&netcfg.OwnerReferences, "ForeignCluster") == nil {
		fc, err := foreignclusterutils.GetForeignClusterByID(ctx, tec.Client, clusterID)
		if client.IgnoreNotFound(err) != nil {
			klog.Errorf("Failed to retrieve ForeignCluster associated with NetworkConfig %q: %v", klog.KObj(netcfg), err)
			return err
		}

		// It could happen that the networkconfig is created before the foreigncluster.
		if err != nil {
			klog.V(4).Infof("Failed to retrieve ForeignCluster associated with NetworkConfig %q: %v", klog.KObj(netcfg), err)
			return nil
		}

		utilruntime.Must(controllerutil.SetOwnerReference(fc, netcfg, tec.Scheme))
		if err := tec.Update(ctx, netcfg); err != nil {
			klog.Errorf("An error occurred while assigning the owner reference to the remote NetworkConfig %q: %v", klog.KObj(netcfg), err)
			return err
		}
	}

	return nil
}

func (tec *TunnelEndpointCreator) enforceRemoteNetConfigStatus(ctx context.Context, netcfg *netv1alpha1.NetworkConfig) error {
	tracer := trace.FromContext(ctx)
	clusterID := netcfg.Labels[liqoconst.ReplicationOriginLabel]

	// Get the CIDR remappings
	podCIDR, externalCIDR, err := tec.IPManager.GetSubnetsPerCluster(netcfg.Spec.PodCIDR, netcfg.Spec.ExternalCIDR, clusterID)
	if err != nil {
		klog.Errorf("An error occurred while getting a new subnet for resource %q: %v", klog.KObj(netcfg), err)
		return err
	}
	tracer.Step("CIDR remappings retrieval")

	// Set the default values in case the CIDRs have not been remapped
	if podCIDR == netcfg.Spec.PodCIDR {
		podCIDR = liqoconst.DefaultCIDRValue
	}
	if externalCIDR == netcfg.Spec.ExternalCIDR {
		externalCIDR = liqoconst.DefaultCIDRValue
	}

	// Update the status fields
	original := netcfg.Status.DeepCopy()
	netcfg.Status.Processed = true
	netcfg.Status.PodCIDRNAT = podCIDR
	netcfg.Status.ExternalCIDRNAT = externalCIDR

	// Avoid performing updates in case it is not necessary
	if !reflect.DeepEqual(original, netcfg.Status) {
		if err := tec.Status().Update(ctx, netcfg); err != nil {
			klog.Errorf("An error occurred while updating the status of remote NetworkConfig %q: %v", klog.KObj(netcfg), err)
			return err
		}
	}

	return nil
}

func (tec *TunnelEndpointCreator) enforceTunnelEndpoint(ctx context.Context, local, remote *netv1alpha1.NetworkConfig) error {
	tracer := trace.FromContext(ctx)

	// At this point we have all the necessary parameters to create the tunnelEndpoint resource
	param := &networkParam{
		remoteCluster:         local.Spec.RemoteCluster,
		remoteEndpointIP:      remote.Spec.EndpointIP,
		remotePodCIDR:         remote.Spec.PodCIDR,
		remoteNatPodCIDR:      remote.Status.PodCIDRNAT,
		remoteExternalCIDR:    remote.Spec.ExternalCIDR,
		remoteNatExternalCIDR: remote.Status.ExternalCIDRNAT,
		localNatPodCIDR:       local.Status.PodCIDRNAT,
		localEndpointIP:       local.Spec.EndpointIP,
		localPodCIDR:          local.Spec.PodCIDR,
		localExternalCIDR:     local.Spec.ExternalCIDR,
		localNatExternalCIDR:  local.Status.ExternalCIDRNAT,
		backendType:           remote.Spec.BackendType,
		backendConfig:         remote.Spec.BackendConfig,
	}

	// Try to get the tunnelEndpoint, which may not exist
	_, found, err := tec.GetTunnelEndpoint(ctx, param.remoteCluster.ClusterID, local.GetNamespace())
	tracer.Step("TunnelEndpoint retrieval")
	if err != nil {
		klog.Errorf("an error occurred while getting resource tunnelEndpoint for cluster %s: %s",
			param.remoteCluster, err)
		return err
	}

	if !found {
		controllerRef := metav1.GetControllerOf(local)
		defer tracer.Step("TunnelEndpoint creation")
		return tec.createTunnelEndpoint(ctx, param, controllerRef, local.GetNamespace(), local)
	}

	defer tracer.Step("TunnelEndpoint update")
	return tec.updateSpecTunnelEndpoint(ctx, param, local.GetNamespace())
}

func (tec *TunnelEndpointCreator) updateSpecTunnelEndpoint(ctx context.Context, param *networkParam, namespace string) error {
	var tep *netv1alpha1.TunnelEndpoint
	var found bool
	var err error
	// here we recover from conflicting resource versions
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		tep, found, err = tec.GetTunnelEndpoint(ctx, param.remoteCluster.ClusterID, namespace)
		if err != nil {
			return err
		}
		if !found {
			return apierrors.NewNotFound(netv1alpha1.TunnelEndpointGroupResource,
				strings.Join([]string{"tunnelEndpoint for cluster:", param.remoteCluster.ClusterID}, " "))
		}

		original := tep.Spec.DeepCopy()
		tec.fillTunnelEndpointSpec(tep, param)

		// Avoid performing updates in case it is not necessary
		if !reflect.DeepEqual(original, tep.Spec) {
			err = tec.Update(ctx, tep)
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

func (tec *TunnelEndpointCreator) createTunnelEndpoint(ctx context.Context, param *networkParam,
	ownerRef *metav1.OwnerReference, namespace string, localNet *netv1alpha1.NetworkConfig) error {
	// here we create it
	tep := &netv1alpha1.TunnelEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      foreignclusterutils.UniqueName(&param.remoteCluster),
			Namespace: namespace,
			Labels: map[string]string{
				liqoconst.ClusterIDLabelName: param.remoteCluster.ClusterID,
			},
		},
	}

	if err := controllerutil.SetOwnerReference(localNet, tep, tec.Scheme); err != nil {
		klog.Errorf("an error occurred while setting owner reference to resource %s: %v", tep.Name, err)
		return err
	}

	if ownerRef != nil {
		tep.OwnerReferences = append(tep.OwnerReferences, *ownerRef)
	}

	tec.fillTunnelEndpointSpec(tep, param)

	if err := tec.Create(ctx, tep); err != nil {
		klog.Errorf("an error occurred while creating resource %s of type %s: %s",
			tep.Name, netv1alpha1.TunnelEndpointGroupResource, err)
		return err
	}

	klog.Infof("resource %s of type %s created", tep.Name, netv1alpha1.TunnelEndpointGroupResource)

	return nil
}

func (tec *TunnelEndpointCreator) fillTunnelEndpointSpec(tep *netv1alpha1.TunnelEndpoint, param *networkParam) {
	tep.Spec.ClusterID = param.remoteCluster.ClusterID
	tep.Spec.LocalPodCIDR = param.localPodCIDR
	tep.Spec.LocalExternalCIDR = param.localExternalCIDR
	tep.Spec.LocalNATPodCIDR = param.localNatPodCIDR
	tep.Spec.LocalNATExternalCIDR = param.localNatExternalCIDR
	tep.Spec.RemotePodCIDR = param.remotePodCIDR
	tep.Spec.RemoteNATPodCIDR = param.remoteNatPodCIDR
	tep.Spec.RemoteExternalCIDR = param.remoteExternalCIDR
	tep.Spec.RemoteNATExternalCIDR = param.remoteNatExternalCIDR
	tep.Spec.EndpointIP = param.remoteEndpointIP
	tep.Spec.BackendType = param.backendType
	tep.Spec.BackendConfig = param.backendConfig
}

// GetTunnelEndpoint retrieves the tunnelEndpoint resource related to a cluster.
func (tec *TunnelEndpointCreator) GetTunnelEndpoint(ctx context.Context, destinationClusterID, namespace string) (
	*netv1alpha1.TunnelEndpoint,
	bool,
	error) {
	clusterID := destinationClusterID
	tunEndpointList := &netv1alpha1.TunnelEndpointList{}
	labels := client.MatchingLabels{liqoconst.ClusterIDLabelName: clusterID}
	err := tec.List(ctx, tunEndpointList, labels, client.InNamespace(namespace))
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

func (tec *TunnelEndpointCreator) deleteTunEndpoint(ctx context.Context, netConfig *netv1alpha1.NetworkConfig) error {
	tep, found, err := tec.GetTunnelEndpoint(ctx, netConfig.Spec.RemoteCluster.ClusterID, netConfig.GetNamespace())
	if err != nil {
		return err
	}
	if !found {
		klog.Infof("tunnelendpoint resource for cluster %s not found", netConfig.Spec.RemoteCluster)
		return nil
	}
	err = tec.Delete(ctx, tep)
	if err != nil {
		return fmt.Errorf("unable to delete endpoint %s in namespace %s : %w", tep.Name, tep.Namespace, err)
	}
	klog.Infof("resource %s of type %s for remote cluster %s has been removed",
		tep.Name, netv1alpha1.GroupVersion.String(), netConfig.Spec.RemoteCluster)
	return nil
}
