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
	"github.com/go-logr/logr"
	netv1alpha1 "github.com/liqotech/liqo/api/net/v1alpha1"
	liqonetOperator "github.com/liqotech/liqo/pkg/liqonet"
	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"os"
	"os/signal"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// TunnelController reconciles a TunnelEndpoint object
type TunnelController struct {
	client.Client
	Log                          logr.Logger
	Scheme                       *runtime.Scheme
	Recorder                     record.EventRecorder
	TunnelIFacesPerRemoteCluster map[string]int
	RetryTimeout                 time.Duration
}

// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch

func (r *TunnelController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("endpoint", req.NamespacedName)
	var endpoint netv1alpha1.TunnelEndpoint
	//name of our finalizer
	tunnelEndpointFinalizer := "tunnelEndpointFinalizer.net.liqo.io"
	if err := r.Get(ctx, req.NamespacedName, &endpoint); err != nil {
		log.Error(err, "unable to fetch endpoint, probably it has been deleted")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	//we wait for the resource to be ready. The resource is created in two steps, firt the spec and metadata fields
	//then the status field. so we wait for the status to be ready.
	if endpoint.Status.Phase != "Ready" {
		log.Info("the resource", "with name", endpoint.Name, "is not ready")
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
	}
	// examine DeletionTimestamp to determine if object is under deletion
	if endpoint.ObjectMeta.DeletionTimestamp.IsZero() {
		if !liqonetOperator.ContainsString(endpoint.ObjectMeta.Finalizers, tunnelEndpointFinalizer) {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object. This is equivalent
			// registering our finalizer.
			endpoint.ObjectMeta.Finalizers = append(endpoint.Finalizers, tunnelEndpointFinalizer)
			if err := r.Update(ctx, &endpoint); err != nil {
				log.Error(err, "unable to update endpoint")
				return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
			}
		}
	} else {
		//the object is being deleted
		if liqonetOperator.ContainsString(endpoint.Finalizers, tunnelEndpointFinalizer) {
			if err := liqonetOperator.RemoveGreTunnel(&endpoint); err != nil {
				//record an event and return
				r.Recorder.Event(&endpoint, "Warning", "Processing", err.Error())
				return ctrl.Result{}, err
			}
			r.Recorder.Event(&endpoint, "Normal", "Processing", "tunnel network interface removed")
			//safe to do, even if the key does not exist in the map
			delete(r.TunnelIFacesPerRemoteCluster, endpoint.Spec.ClusterID)
			log.Info("tunnel iface removed")
			//remove the finalizer from the list and update it.
			endpoint.Finalizers = liqonetOperator.RemoveString(endpoint.Finalizers, tunnelEndpointFinalizer)
			if err := r.Update(ctx, &endpoint); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
	}
	//try to install the GRE tunnel if it does not exist
	iFaceIndex, iFaceName, err := liqonetOperator.InstallGreTunnel(&endpoint)
	if err != nil {
		log.Error(err, "unable to create the gre tunnel")
		r.Recorder.Event(&endpoint, "Warning", "Processing", err.Error())
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
	}
	r.Recorder.Event(&endpoint, "Normal", "Processing", "tunnel network interface installed")
	log.Info("gre tunnel installed", "index", iFaceIndex, "name", iFaceName)
	//save the IFace index in the map
	r.TunnelIFacesPerRemoteCluster[endpoint.Spec.ClusterID] = iFaceIndex
	log.Info("installed gretunel with index: " + iFaceName)

	//update the status of CR if needed
	//here we recover from conflicting resource versions
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toBeUpdated := false
		if endpoint.Status.TunnelIFaceName != iFaceName {
			endpoint.Status.TunnelIFaceName = iFaceName
			toBeUpdated = true
		}
		if endpoint.Status.TunnelIFaceIndex != iFaceIndex {
			endpoint.Status.TunnelIFaceIndex = iFaceIndex
			toBeUpdated = true
		}
		if toBeUpdated {
			err = r.Status().Update(context.Background(), &endpoint)
			return err
		}
		return nil
	})
	if retryError != nil {
		log.Error(retryError, "unable to create the gre tunnel")
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, retryError
	}
	return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
}

//used to remove all the tunnel interfaces when the controller is closed
//it does not return an error, but just logs them, cause we can not recover from
//them at exit time
func (r *TunnelController) RemoveAllTunnels() {
	logger := r.Log.WithName("RemoveAllTunnels")
	for _, ifaceIndex := range r.TunnelIFacesPerRemoteCluster {
		existingIface, err := netlink.LinkByIndex(ifaceIndex)
		if err == nil {
			//Remove the existing gre interface
			if err = netlink.LinkDel(existingIface); err != nil {
				logger.Error(err, "unable to delete the iface:", "ifaceIndex", ifaceIndex, "ifaceName", existingIface.Attrs().Name)
			}
		} else {
			logger.Error(err, "unable to retrive the iface:", "index", ifaceIndex)
		}
	}
}

// SetupSignalHandlerForRouteOperator registers for SIGTERM, SIGINT, SIGKILL. A stop channel is returned
// which is closed on one of these signals.
func (r *TunnelController) SetupSignalHandlerForTunnelOperator() (stopCh <-chan struct{}) {
	logger := r.Log.WithName("Tunnel Operator Signal Handler")
	stop := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, shutdownSignals...)
	go func(r *TunnelController) {
		sig := <-c
		logger.Info("received ", "signal", sig.String())
		r.RemoveAllTunnels()
		<-c
		close(stop)
	}(r)
	return stop
}

func (r *TunnelController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1alpha1.TunnelEndpoint{}).
		Complete(r)
}
