// Copyright 2019-2025 The Liqo Authors
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

package modules

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	"github.com/liqotech/liqo/pkg/ipam"
	clientoperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/client-operator"
	configuration "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/configuration"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/remapping"
	externalnetworkroute "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/route"
	serveroperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/server-operator"
	wggatewaycontrollers "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/wireguard"
	internalclientcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network/client-controller"
	internalconfigurationcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network/configuration-controller"
	gwmasqbypass "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network/gw-masq-bypass"
	internalfabriccontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network/internalfabric-controller"
	nodecontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network/node-controller"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network/route"
	internalservercontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network/server-controller"
	ipctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/ip-controller"
	networkctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/network-controller"
	dynamicutils "github.com/liqotech/liqo/pkg/utils/dynamic"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
)

// NetworkingOption defines the options to setup the Networking module.
type NetworkingOption struct {
	DynClient dynamic.Interface
	Factory   *dynamicutils.RunnableFactory

	LiqoNamespace string
	IpamClient    ipam.IPAMClient

	GatewayServerResources         []string
	GatewayClientResources         []string
	WgGatewayServerClusterRoleName string
	WgGatewayClientClusterRoleName string
	NetworkWorkers                 int
	IPWorkers                      int
	FabricFullMasquerade           bool
	GwmasqbypassEnabled            bool

	GenevePort uint16
}

// SetupNetworkingModule setup the networking module and initializes its controllers .
func SetupNetworkingModule(ctx context.Context, mgr manager.Manager, uncachedClient client.Client, opts *NetworkingOption) error {
	// Initialize reserved networks
	if err := initializeReservedNetworks(ctx, uncachedClient, opts.IpamClient); err != nil {
		klog.Errorf("Unable to initialize reserved networks: %v", err)
		return err
	}

	networkReconciler := networkctrl.NewNetworkReconciler(mgr.GetClient(), mgr.GetScheme(), opts.IpamClient)
	if err := networkReconciler.SetupWithManager(mgr, opts.NetworkWorkers); err != nil {
		klog.Errorf("Unable to start the networkReconciler: %v", err)
		return err
	}

	ipReconciler := ipctrl.NewIPReconciler(mgr.GetClient(), mgr.GetScheme(), opts.IpamClient)
	if err := ipReconciler.SetupWithManager(mgr, opts.IPWorkers); err != nil {
		klog.Errorf("Unable to start the ipReconciler: %v", err)
		return err
	}

	cfgReconciler := configuration.NewConfigurationReconciler(mgr.GetClient(), mgr.GetScheme(),
		mgr.GetEventRecorderFor("configuration-controller"))
	if err := cfgReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to create controller configurationReconciler: %s", err)
		return err
	}

	extCfgReconciler := externalnetworkroute.NewConfigurationReconciler(mgr.GetClient(), mgr.GetScheme(),
		mgr.GetEventRecorderFor("external-configuration-controller"))
	if err := extCfgReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to create controller externalConfigurationReconciler: %s", err)
		return err
	}

	intPodReconciler := route.NewPodReconciler(mgr.GetClient(), mgr.GetScheme(),
		mgr.GetEventRecorderFor("internal-pod-controller"), &route.Options{Namespace: opts.LiqoNamespace})
	if err := intPodReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("unable to create controller internalPodReconciler: %s", err)
		return err
	}

	wgServerRec := wggatewaycontrollers.NewWgGatewayServerReconciler(mgr.GetClient(), mgr.GetScheme(),
		mgr.GetEventRecorderFor("wg-gateway-server-controller"),
		opts.WgGatewayServerClusterRoleName)
	if err := wgServerRec.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the wgGatewayServerReconciler: %v", err)
		return err
	}

	wgClientRec := wggatewaycontrollers.NewWgGatewayClientReconciler(mgr.GetClient(), mgr.GetScheme(),
		mgr.GetEventRecorderFor("wg-gateway-client-controller"),
		opts.WgGatewayClientClusterRoleName)
	if err := wgClientRec.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the wgGatewayClientReconciler: %v", err)
		return err
	}

	serverReconciler := serveroperator.NewServerReconciler(mgr.GetClient(),
		opts.DynClient, opts.Factory, mgr.GetScheme(),
		mgr.GetEventRecorderFor("server-controller"),
		opts.GatewayServerResources)
	if err := serverReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the serverReconciler: %v", err)
		return err
	}

	clientReconciler := clientoperator.NewClientReconciler(mgr.GetClient(),
		opts.DynClient, opts.Factory, mgr.GetScheme(),
		mgr.GetEventRecorderFor("client-controller"),
		opts.GatewayClientResources)
	if err := clientReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the clientReconciler: %v", err)
		return err
	}

	internalServerReconciler := internalservercontroller.NewServerReconciler(mgr.GetClient(), mgr.GetScheme())
	if err := internalServerReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the internalServerReconciler: %v", err)
		return err
	}

	internalClientReconciler := internalclientcontroller.NewClientReconciler(mgr.GetClient(), mgr.GetScheme())
	if err := internalClientReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the internalClientReconciler: %v", err)
		return err
	}

	internalFabricReconciler := internalfabriccontroller.NewInternalFabricReconciler(mgr.GetClient(), mgr.GetScheme())
	if err := internalFabricReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the internalFabricReconciler: %v", err)
		return err
	}

	configurationReconciler := internalconfigurationcontroller.NewConfigurationReconciler(mgr.GetClient(), mgr.GetScheme(),
		&internalconfigurationcontroller.Options{
			FullMasqueradeEnabled: opts.FabricFullMasquerade,
		})
	if err := configurationReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the configurationReconciler: %v", err)
		return err
	}

	// Before starting the Node reconciler, make sure that there are no "orphan" InternalNode resources.
	if err := nodecontroller.SyncInternalNodes(ctx, uncachedClient); err != nil {
		klog.Errorf("Unable to perform InternalNode synchronization: %v", err)
		return err
	}

	nodeReconciler := nodecontroller.NewNodeReconciler(mgr.GetClient(), mgr.GetScheme(), opts.LiqoNamespace)
	if err := nodeReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the nodeReconciler: %v", err)
		return err
	}

	internalNodeReconciler := route.NewInternalNodeReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("internal-node-controller"),
		&route.Options{Namespace: opts.LiqoNamespace},
	)
	if err := internalNodeReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the internalNodeReconciler: %v", err)
		return err
	}

	ipMappingReconciler := remapping.NewIPReconciler(mgr.GetClient(), mgr.GetScheme())
	if err := ipMappingReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the ipMappingReconciler: %v", err)
		return err
	}

	remappingReconciler, err := remapping.NewRemappingReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("remapping-controller"),
	)
	if err != nil {
		klog.Errorf("Unable to initialize the remappingReconciler: %v", err)
		return err
	}
	if err := remappingReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the remappingReconciler: %v", err)
		return err
	}

	if opts.GwmasqbypassEnabled {
		gwmasqbypassReconciler := gwmasqbypass.NewPodReconciler(
			mgr.GetClient(),
			mgr.GetScheme(),
			mgr.GetEventRecorderFor("gw-masq-bypass-controller"),
			&gwmasqbypass.Options{
				Namespace:  opts.LiqoNamespace,
				GenevePort: opts.GenevePort,
			},
		)
		if err := gwmasqbypassReconciler.SetupWithManager(mgr); err != nil {
			klog.Errorf("Unable to start the gw-masq-bypass reconciler: %v", err)
			return err
		}
	}

	return nil
}

func initializeReservedNetworks(ctx context.Context, cl client.Client, ipamClient ipam.IPAMClient) error {
	var networksToReserve []ipamv1alpha1.Network

	// PodCIDR is a special case of reserved network
	podCidr, err := ipamutils.GetPodCIDRNetwork(ctx, cl, corev1.NamespaceAll)
	if err != nil {
		return err
	}
	networksToReserve = append(networksToReserve, *podCidr)

	// ServiceCIDR is a special case of reserved network
	serviceCidr, err := ipamutils.GetServiceCIDRNetwork(ctx, cl, corev1.NamespaceAll)
	if err != nil {
		return err
	}
	networksToReserve = append(networksToReserve, *serviceCidr)

	// Get the reserved networks
	reservedNetworks, err := ipamutils.GetReservedSubnetNetworks(ctx, cl)
	if err != nil {
		return err
	}
	networksToReserve = append(networksToReserve, reservedNetworks...)

	// Reserve the networks and fill their status CIDR.
	for i := range networksToReserve {
		nw := &networksToReserve[i]

		// If the status CIDR is already set, we do not need to reserve the network
		// as it will be reserved when the ipam server is initialized.
		if nw.Status.CIDR != "" {
			continue
		}

		if ipamClient == nil {
			nw.Status.CIDR = nw.Spec.CIDR
		} else {
			// First check if the network is already reserved
			res, err := ipamClient.NetworkIsAvailable(ctx, &ipam.NetworkAvailableRequest{
				Cidr: nw.Spec.CIDR.String(),
			})
			if err != nil {
				return fmt.Errorf("IPAM: %w", err)
			}

			if res.Available {
				// Network is not reserved, reserve it
				_, err := ipamClient.NetworkAcquire(ctx, &ipam.NetworkAcquireRequest{
					Cidr:         nw.Spec.CIDR.String(),
					Immutable:    true,
					PreAllocated: nw.Spec.PreAllocated,
				})
				if err != nil {
					return fmt.Errorf("IPAM: %w", err)
				}
			}

			// Since reserved network must not be remapped (immutable), we can set the status CIDR to the spec CIDR
			nw.Status.CIDR = nw.Spec.CIDR
		}

		if err := cl.Status().Update(ctx, nw); err != nil {
			return fmt.Errorf("unable to update the reserved network %s: %w", nw.Name, err)
		}
		klog.Infof("Updated reserved Network %q status (spec: %s | status: %s)", client.ObjectKeyFromObject(nw), nw.Spec.CIDR, nw.Status.CIDR)
	}

	klog.Info("Reserved networks initialized")
	return nil
}
