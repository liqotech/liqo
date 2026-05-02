// Copyright 2019-2026 The Liqo Authors
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

package peer

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	nwforge "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/authenticate"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/network"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/resourceslice"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// Options encapsulates the arguments of the peer command.
type Options struct {
	LocalFactory   *factory.Factory
	RemoteFactory  *factory.Factory
	Timeout        time.Duration
	SkipValidation bool

	// Networking options
	NetworkingDisabled          bool
	ServerServiceLocation       *argsutils.StringEnum
	ServerServiceType           *argsutils.StringEnum
	ServerServicePort           int32
	ServerServiceNodePort       int32
	ServerServiceLoadBalancerIP string
	ClientConnectAddress        string
	ClientConnectPort           int32
	MTU                         int

	// Authentication options
	CreateResourceSlice bool
	ResourceSliceClass  string
	InBand              bool
	ProxyURL            string

	// Offloading options
	CreateVirtualNode bool
	MultiVirtualNode  bool
	CPU               string
	Memory            string
	Pods              string
	OtherResources    map[string]string
}

// NewOptions returns a new Options struct.
func NewOptions(localFactory *factory.Factory) *Options {
	return &Options{
		LocalFactory: localFactory,
		ServerServiceLocation: argsutils.NewEnum(
			[]string{string(liqov1beta1.ConsumerRole), string(liqov1beta1.ProviderRole)},
			string(nwforge.DefaultGwServerLocation)),
		ServerServiceType: argsutils.NewEnum(
			[]string{string(corev1.ServiceTypeLoadBalancer), string(corev1.ServiceTypeNodePort), string(corev1.ServiceTypeClusterIP)},
			string(nwforge.DefaultGwServerServiceType)),
	}
}

// RunPeer implements the peer command.
func (o *Options) RunPeer(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// To ease the experience for most users, we disable the namespace and remote-namespace flags
	// so that resources are created according to the default Liqo logic.
	// Advanced users can use the individual commands (e.g., liqoctl network connect, liqoctl network disconnect, etc..) to
	// customize the namespaces according to their needs (e.g., networking resources in a specific namespace).
	o.LocalFactory.Namespace = ""
	o.RemoteFactory.Namespace = ""

	// Ensure networking
	if !o.NetworkingDisabled {
		if err := ensureNetworking(ctx, o); err != nil {
			o.LocalFactory.PrinterGlobal.Error.Printfln("Unable to ensure networking: %v", err)
			return err
		}
	}

	// Ensure authentication
	if err := ensureAuthentication(ctx, o); err != nil {
		o.LocalFactory.PrinterGlobal.Error.Printfln("Unable to ensure authentication: %v", err)
		return err
	}

	// Ensure offloading
	if o.CreateResourceSlice {
		if err := ensureOffloading(ctx, o); err != nil {
			o.LocalFactory.PrinterGlobal.Error.Printfln("Unable to ensure offloading: %v", err)
			return err
		}
	}

	return nil
}

func ensureNetworking(ctx context.Context, o *Options) error {
	localFactory := o.LocalFactory
	remoteFactory := o.RemoteFactory

	// Invert the local and remote factories if the server service position is Consumer.
	if o.ServerServiceLocation.Value == string(liqov1beta1.ConsumerRole) {
		localFactory = o.RemoteFactory
		remoteFactory = o.LocalFactory
	}

	networkOptions := network.Options{
		LocalFactory:  localFactory,
		RemoteFactory: remoteFactory,

		Timeout:        o.Timeout,
		Wait:           true,
		SkipValidation: o.SkipValidation,

		ServerGatewayType:           nwforge.DefaultGwServerType,
		ServerTemplateName:          nwforge.DefaultGwServerTemplateName,
		ServerTemplateNamespace:     remoteFactory.LiqoNamespace,
		ServerServiceType:           o.ServerServiceType,
		ServerServicePort:           o.ServerServicePort,
		ServerServiceNodePort:       o.ServerServiceNodePort,
		ServerServiceLoadBalancerIP: o.ServerServiceLoadBalancerIP,

		ClientGatewayType:       nwforge.DefaultGwClientType,
		ClientTemplateName:      nwforge.DefaultGwClientTemplateName,
		ClientTemplateNamespace: localFactory.LiqoNamespace,
		ClientConnectAddress:    o.ClientConnectAddress,
		ClientConnectPort:       o.ClientConnectPort,

		MTU:                o.MTU,
		DisableSharingKeys: false,
	}

	if err := networkOptions.RunConnect(ctx); err != nil {
		return err
	}

	return nil
}

func ensureAuthentication(ctx context.Context, o *Options) error {
	authOptions := authenticate.Options{
		LocalFactory:  o.LocalFactory,
		RemoteFactory: o.RemoteFactory,
		Timeout:       o.Timeout,

		InBand:   o.InBand,
		ProxyURL: o.ProxyURL,
	}

	if err := authOptions.RunAuthenticate(ctx); err != nil {
		return err
	}

	return nil
}

func ensureOffloading(ctx context.Context, o *Options) error {
	providerClusterID, err := liqoutils.GetClusterIDWithControllerClient(ctx, o.RemoteFactory.CRClient, o.RemoteFactory.LiqoNamespace)
	if err != nil {
		return err
	}

	providerClusterIDFlag := argsutils.ClusterIDFlags{}
	if err := providerClusterIDFlag.Set(string(providerClusterID)); err != nil {
		return err
	}

	nsManager := tenantnamespace.NewManager(o.LocalFactory.KubeClient, o.LocalFactory.CRClient.Scheme())

	if o.MultiVirtualNode {
		return ensureOffloadingPerNode(ctx, o, providerClusterID, providerClusterIDFlag, nsManager)
	}

	rsOptions := resourceslice.Options{
		CreateOptions: &rest.CreateOptions{
			Factory: o.LocalFactory,
			Name:    string(providerClusterID),
		},

		NamespaceManager:           nsManager,
		RemoteClusterID:            providerClusterIDFlag,
		Class:                      o.ResourceSliceClass,
		DisableVirtualNodeCreation: !o.CreateVirtualNode,

		CPU:            o.CPU,
		Memory:         o.Memory,
		Pods:           o.Pods,
		OtherResources: o.OtherResources,
	}

	return rsOptions.HandleCreate(ctx)
}

func ensureOffloadingPerNode(ctx context.Context, o *Options, providerClusterID liqov1beta1.ClusterID,
	providerClusterIDFlag argsutils.ClusterIDFlags, nsManager tenantnamespace.Manager) error {
	var nodeList corev1.NodeList
	if err := o.RemoteFactory.CRClient.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("unable to list remote cluster nodes: %w", err)
	}

	var workerCount int
	for i := range nodeList.Items {
		if !isControlPlaneNode(&nodeList.Items[i]) {
			workerCount++
		}
	}
	if workerCount == 0 {
		return fmt.Errorf("no worker nodes found in remote cluster %q", providerClusterID)
	}

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if isControlPlaneNode(node) {
			continue
		}
		rsName := fmt.Sprintf("%s-%s", providerClusterID, node.Name)

		rsOptions := resourceslice.Options{
			CreateOptions: &rest.CreateOptions{
				Factory: o.LocalFactory,
				Name:    rsName,
			},

			NamespaceManager:           nsManager,
			RemoteClusterID:            providerClusterIDFlag,
			Class:                      o.ResourceSliceClass,
			DisableVirtualNodeCreation: !o.CreateVirtualNode,
			NodeName:                   node.Name,

			CPU:            o.CPU,
			Memory:         o.Memory,
			Pods:           o.Pods,
			OtherResources: o.OtherResources,
		}

		if err := rsOptions.HandleCreate(ctx); err != nil {
			return fmt.Errorf("unable to create ResourceSlice for node %q: %w", node.Name, err)
		}
	}

	return nil
}

func isControlPlaneNode(node *corev1.Node) bool {
	labels := node.GetLabels()
	_, hasControlPlane := labels["node-role.kubernetes.io/control-plane"]
	_, hasMaster := labels["node-role.kubernetes.io/master"]
	_, hasControlplaneLegacy := labels["node-role.kubernetes.io/controlplane"]
	if hasControlPlane || hasMaster || hasControlplaneLegacy {
		return true
	}
	for i := range node.Spec.Taints {
		t := &node.Spec.Taints[i]
		if t.Key == "node-role.kubernetes.io/control-plane" || t.Key == "node-role.kubernetes.io/master" {
			return true
		}
	}
	return false
}
