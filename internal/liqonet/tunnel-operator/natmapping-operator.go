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

package tunneloperator

import (
	"context"
	"fmt"
	"sync"

	"github.com/containernetworking/plugins/pkg/ns"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqonet/iptables"
)

// NatMappingController reconciles a NatMapping object.
type NatMappingController struct {
	client.Client
	iptables.IPTHandler
	readyClustersMutex *sync.Mutex
	readyClusters      map[string]struct{}
	gatewayNetns       ns.NetNS
}

//+kubebuilder:rbac:groups=net.liqo.io,resources=natmappings,verbs=get;list;watch;create;update;patch;delete

// Reconcile function handles requests made on NatMapping resource
// by guaranteeing the proper set of DNAT rules are updated.
func (npc *NatMappingController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var nm netv1alpha1.NatMapping

	if err := npc.Get(ctx, req.NamespacedName, &nm); err != nil {
		return result, client.IgnoreNotFound(err)
	}
	// There's no need of a pre-delete logic since IPTables rules for cluster are removed by the
	// tunnel-operator after the un-peer.

	// The following logic has to be executed in the custom network namespace,
	// and not on the root namespace. Therefore it must be defined in a closure
	// and then used as parameter of method Do of netNs
	if err := npc.gatewayNetns.Do(func(netNamespace ns.NetNS) error {
		// Is the remote cluster tunnel ready? If not, do nothing
		npc.readyClustersMutex.Lock()
		defer npc.readyClustersMutex.Unlock()
		if _, ready := npc.readyClusters[nm.Spec.ClusterID]; !ready {
			return fmt.Errorf("tunnel for cluster {%s} is not ready", nm.Spec.ClusterID)
		}
		if err := npc.IPTHandler.EnsurePreroutingRulesPerNatMapping(&nm); err != nil {
			klog.Errorf("unable to ensure prerouting rules for cluster {%s}: %s",
				nm.Spec.ClusterID, err.Error())
			return err
		}
		return nil
	}); err != nil {
		return result, err
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (npc *NatMappingController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&netv1alpha1.NatMapping{}).
		Complete(npc)
}

// NewNatMappingController returns a NAT mapping controller istance.
func NewNatMappingController(cl client.Client, readyClustersMutex *sync.Mutex,
	readyClusters map[string]struct{}, gatewayNetns ns.NetNS) (*NatMappingController, error) {
	iptablesHandler, err := iptables.NewIPTHandler()
	if err != nil {
		return nil, err
	}
	return &NatMappingController{
		Client:             cl,
		IPTHandler:         iptablesHandler,
		readyClustersMutex: readyClustersMutex,
		readyClusters:      readyClusters,
		gatewayNetns:       gatewayNetns,
	}, nil
}
