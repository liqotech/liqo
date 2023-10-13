// Copyright 2019-2023 The Liqo Authors
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

package tunneloperator

import (
	"context"
	"fmt"
	"sync"

	"github.com/containernetworking/plugins/pkg/ns"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqoipset "github.com/liqotech/liqo/pkg/liqonet/ipset"
	liqoiptables "github.com/liqotech/liqo/pkg/liqonet/iptables"
	liqovk "github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// OffloadedPodController reconciles an offloaded Pod object.
type OffloadedPodController struct {
	client.Client
	liqoiptables.IPTHandler
	*liqoipset.IPSHandler

	// Liqo Gateway network namespace
	gatewayNetns ns.NetNS

	// Local cache of podInfo objects
	podsInfo *sync.Map

	// Local cache of endpointsliceInfo objects
	endpointslicesInfo *sync.Map
}

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;

// NewOffloadedPodController instantiates and initializes the offloaded pod controller.
func NewOffloadedPodController(cl client.Client, gatewayNetns ns.NetNS, podsInfo, endpointslicesInfo *sync.Map) (*OffloadedPodController, error) {
	// Create the IPTables handler
	iptablesHandler, err := liqoiptables.NewIPTHandler()
	if err != nil {
		return nil, err
	}
	// Create the IPSet handler
	ipsetHandler := liqoipset.NewIPSHandler()
	// Create and return the controller
	return &OffloadedPodController{
		Client:             cl,
		IPTHandler:         iptablesHandler,
		IPSHandler:         &ipsetHandler,
		gatewayNetns:       gatewayNetns,
		podsInfo:           podsInfo,
		endpointslicesInfo: endpointslicesInfo,
	}, nil
}

// Reconcile pods offloaded from other clusters to the local one.
func (r *OffloadedPodController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var ensureIptablesRules = func(netns ns.NetNS) error {
		return r.EnsureRulesForClustersForwarding(r.podsInfo, r.endpointslicesInfo, r.IPSHandler)
	}
	nsName := req.NamespacedName
	klog.V(3).Infof("Reconcile Pod %q", nsName)

	pod := corev1.Pod{}
	if err := r.Get(ctx, nsName, &pod); err != nil {
		if apierror.IsNotFound(err) {
			// Pod not found, podInfo object found: delete podInfo object
			if value, ok := r.podsInfo.LoadAndDelete(nsName); ok {
				klog.V(3).Infof("Pod %q not found: ensuring updated iptables rules", nsName)

				// Soft delete object
				podInfo := value.(liqoiptables.PodInfo)
				podInfo.Deleting = true
				r.podsInfo.Store(nsName, podInfo)

				if err := r.gatewayNetns.Do(ensureIptablesRules); err != nil {
					return ctrl.Result{}, fmt.Errorf("error while ensuring iptables rules: %w", err)
				}

				// Hard delete object
				r.podsInfo.Delete(nsName)
			}

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Build podInfo object
	podInfo := liqoiptables.PodInfo{
		PodIP:           pod.Status.PodIP,
		RemoteClusterID: pod.Labels[liqovk.LiqoOriginClusterIDKey],
	}

	// Check if the object is under deletion
	if !pod.ObjectMeta.DeletionTimestamp.IsZero() {
		// Pod under deletion: skip creation of iptables rules and return no error
		klog.V(3).Infof("Pod %q under deletion: skipping iptables rules update", nsName)
		return ctrl.Result{}, nil
	}

	// Check if the pod IP is set
	if podInfo.PodIP == "" {
		// Pod IP address not yet set: skip creation of iptables rules and return no error
		klog.V(3).Infof("Pod %q IP address not yet set: skipping iptables rules update", nsName)
		return ctrl.Result{}, nil
	}

	// Store podInfo object
	r.podsInfo.Store(nsName, podInfo)

	// Ensure iptables rules
	klog.V(3).Infof("Ensuring updated iptables rules")
	if err := r.gatewayNetns.Do(ensureIptablesRules); err != nil {
		klog.Errorf("Error while ensuring iptables rules: %w", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OffloadedPodController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
