// Copyright 2019-2024 The Liqo Authors
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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/ipam"
	clientoperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/client-operator"
	configurationcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/configuration-controller"
	externalnetworkcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/externalnetwork-controller"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/remapping"
	externalnetworkroute "github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/route"
	serveroperator "github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/server-operator"
	wggatewaycontrollers "github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/wireguard"
	internalclientcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/internal-network/client-controller"
	internalconfigurationcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/internal-network/configuration-controller"
	internalfabriccontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/internal-network/internalfabric-controller"
	nodecontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/internal-network/node-controller"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/internal-network/route"
	internalservercontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/internal-network/server-controller"
	ipctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/ip-controller"
	networkctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/network-controller"
	dynamicutils "github.com/liqotech/liqo/pkg/utils/dynamic"
)

// NetworkingOption defines the options to setup the Networking module.
type NetworkingOption struct {
	DynClient  dynamic.Interface
	Factory    *dynamicutils.RunnableFactory
	KubeClient kubernetes.Interface

	LiqoNamespace        string
	LocalClusterIdentity *discoveryv1alpha1.ClusterIdentity
	IpamClient           ipam.IpamClient

	GatewayServerResources         []string
	GatewayClientResources         []string
	WgGatewayServerClusterRoleName string
	WgGatewayClientClusterRoleName string
	GatewayServiceType             corev1.ServiceType
	GatewayServicePort             int32
	GatewayMTU                     int
	GatewayProxy                   bool
	NetworkWorkers                 int
	IPWorkers                      int
}

// SetupNetworkingModule setup the networking module and initializes its controllers .
func SetupNetworkingModule(ctx context.Context, mgr manager.Manager, opts *NetworkingOption) error {
	networkReconciler := networkctrl.NewNetworkReconciler(mgr.GetClient(), mgr.GetScheme(), opts.IpamClient)
	if err := networkReconciler.SetupWithManager(mgr, opts.NetworkWorkers); err != nil {
		klog.Errorf("Unable to start the networkReconciler: %v", err)
		return err
	}

	ipReconciler := ipctrl.NewIPReconciler(mgr.GetClient(), mgr.GetScheme(), opts.IpamClient)
	if err := ipReconciler.SetupWithManager(ctx, mgr, opts.IPWorkers); err != nil {
		klog.Errorf("Unable to start the ipReconciler: %v", err)
		return err
	}

	cfgReconciler := configurationcontroller.NewConfigurationReconciler(mgr.GetClient(), mgr.GetScheme(),
		mgr.GetEventRecorderFor("configuration-controller"), opts.IpamClient)
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

	wgServerRec := wggatewaycontrollers.NewWgGatewayServerReconciler(
		mgr.GetClient(), mgr.GetScheme(), opts.WgGatewayServerClusterRoleName)
	if err := wgServerRec.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the wgGatewayServerReconciler: %v", err)
		return err
	}

	wgClientRec := wggatewaycontrollers.NewWgGatewayClientReconciler(mgr.GetClient(), mgr.GetScheme(),
		opts.WgGatewayClientClusterRoleName)
	if err := wgClientRec.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the wgGatewayClientReconciler: %v", err)
		return err
	}

	serverReconciler := serveroperator.NewServerReconciler(mgr.GetClient(),
		opts.DynClient, opts.Factory, mgr.GetScheme(), opts.GatewayServerResources)
	if err := serverReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the serverReconciler: %v", err)
		return err
	}

	clientReconciler := clientoperator.NewClientReconciler(mgr.GetClient(),
		opts.DynClient, opts.Factory, mgr.GetScheme(), opts.GatewayClientResources)
	if err := clientReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the clientReconciler: %v", err)
		return err
	}

	externalNetworkReconciler := externalnetworkcontroller.NewExternalNetworkReconciler(
		mgr.GetClient(), mgr.GetScheme(), opts.KubeClient, opts.LiqoNamespace, opts.LocalClusterIdentity,
		opts.GatewayServiceType, opts.GatewayServicePort, opts.GatewayMTU, opts.GatewayProxy)
	if err := externalNetworkReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the externalNetworkReconciler: %v", err)
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

	configurationReconciler := internalconfigurationcontroller.NewConfigurationReconciler(mgr.GetClient(), mgr.GetScheme())
	if err := configurationReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to start the configurationReconciler: %v", err)
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

	return nil
}
