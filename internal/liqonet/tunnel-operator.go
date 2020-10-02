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
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqonetOperator "github.com/liqotech/liqo/pkg/liqonet"
	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"os"
	"os/signal"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
)

// TunnelController reconciles a TunnelEndpoint object
type TunnelController struct {
	client.Client
	Scheme                       *runtime.Scheme
	Recorder                     record.EventRecorder
	TunnelIFacesPerRemoteCluster map[string]int
	RetryTimeout                 time.Duration
}

// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;update;patch

func (r *TunnelController) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	var endpoint netv1alpha1.TunnelEndpoint
	//name of our finalizer
	tunnelEndpointFinalizer := "tunnelEndpointFinalizer.net.liqo.io"
	if err := r.Get(ctx, req.NamespacedName, &endpoint); err != nil {
		klog.Errorf("unable to fetch resource %s: %s", req.Name, err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	//we wait for the resource to be ready. The resource is created in two steps, firt the spec and metadata fields
	//then the status field. so we wait for the status to be ready.
	if endpoint.Status.Phase != "Ready" {
		klog.Infof("%s -> resource %s is not ready", endpoint.Spec.ClusterID, endpoint.Name)
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
				klog.Errorf("%s -> unable to update resource %s: %s", endpoint.Spec.ClusterID, endpoint.Name, err)
				return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
			}
		}
	} else {
		//the object is being deleted
		if liqonetOperator.ContainsString(endpoint.Finalizers, tunnelEndpointFinalizer) {
			if err := liqonetOperator.RemoveGreTunnel(&endpoint); err != nil {
				//record an event and return
				r.Recorder.Event(&endpoint, "Warning", "Processing", err.Error())
				klog.Errorf("%s -> unable to remove tunnel network interface %s for resource %s: %s", endpoint.Spec.ClusterID, endpoint.Status.TunnelIFaceName, endpoint.Name, err)
				return ctrl.Result{}, err
			}
			r.Recorder.Event(&endpoint, "Normal", "Processing", "tunnel network interface removed")
			//safe to do, even if the key does not exist in the map
			delete(r.TunnelIFacesPerRemoteCluster, endpoint.Spec.ClusterID)
			klog.Infof("%s -> tunnel network interface %s removed for resource %s", endpoint.Spec.ClusterID, endpoint.Status.TunnelIFaceName, endpoint.Name)
			retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := r.Get(ctx, req.NamespacedName, &endpoint); err != nil {
					klog.Errorf("unable to fetch resource %s: %s", req.Name, err)
					return err
				}
				//remove the finalizer from the list and update it.
				endpoint.Finalizers = liqonetOperator.RemoveString(endpoint.Finalizers, tunnelEndpointFinalizer)
				if err := r.Update(ctx, &endpoint); err != nil {
					return err
				}
				return nil
			})
			if retryError != nil {
				klog.Errorf("%s -> unable to update finalizers of resource %s: %s", endpoint.Spec.ClusterID, endpoint.Name, retryError)
				return ctrl.Result{RequeueAfter: r.RetryTimeout}, retryError
			}
			return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
		}
	}
	//try to install the GRE tunnel if it does not exist
	iFaceIndex, iFaceName, err := liqonetOperator.InstallGreTunnel(&endpoint)
	if err != nil {
		klog.Errorf("%s -> unable to create tunnel network interface for resource %s :%s", endpoint.Spec.ClusterID, endpoint.Name, err)
		r.Recorder.Event(&endpoint, "Warning", "Processing", err.Error())
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, err
	}
	r.Recorder.Event(&endpoint, "Normal", "Processing", "tunnel network interface installed")
	klog.Infof("%s -> tunnel network interface with name %s for resource %s created successfully", endpoint.Spec.ClusterID, iFaceName, endpoint.Name)
	//save the IFace index in the map
	r.TunnelIFacesPerRemoteCluster[endpoint.Spec.ClusterID] = iFaceIndex
	//update the status of CR if needed
	//here we recover from conflicting resource versions
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toBeUpdated := false
		if err := r.Get(ctx, req.NamespacedName, &endpoint); err != nil {
			klog.Errorf("unable to fetch resource %s: %s", req.Name, err)
			return err
		}
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
		klog.Errorf("%s -> unable to update status of resource %s: %s", endpoint.Spec.ClusterID, endpoint.Name, retryError)
		return ctrl.Result{RequeueAfter: r.RetryTimeout}, retryError
	}
	return ctrl.Result{RequeueAfter: r.RetryTimeout}, nil
}

//used to remove all the tunnel interfaces when the controller is closed
//it does not return an error, but just logs them, cause we can not recover from
//them at exit time
func (r *TunnelController) RemoveAllTunnels() {
	for clusterID, ifaceIndex := range r.TunnelIFacesPerRemoteCluster {
		existingIface, err := netlink.LinkByIndex(ifaceIndex)
		if err == nil {
			//Remove the existing gre interface
			if err = netlink.LinkDel(existingIface); err != nil {
				klog.Errorf("%s -> unable to delete tunnel network interface with name %s: %s", clusterID, existingIface.Attrs().Name, err)
			}
		} else {
			klog.Errorf("%s -> unable to fetch tunnel network interface with index %d: %s", clusterID, ifaceIndex, err)
		}
	}
}

// SetupSignalHandlerForRouteOperator registers for SIGTERM, SIGINT, SIGKILL. A stop channel is returned
// which is closed on one of these signals.
func (r *TunnelController) SetupSignalHandlerForTunnelOperator() (stopCh <-chan struct{}) {
	stop := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, shutdownSignals...)
	go func(r *TunnelController) {
		sig := <-c
		klog.Infof("received signal %s: cleaning up", sig.String())
		r.RemoveAllTunnels()
		<-c
		close(stop)
	}(r)
	return stop
}

func (r *TunnelController) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			//finalizers are used to check if a resource is being deleted, and perform there the needed actions
			//we don't want to reconcile on the delete of a resource.
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1alpha1.TunnelEndpoint{}).WithEventFilter(resourceToBeProccesedPredicate).
		Complete(r)
}
