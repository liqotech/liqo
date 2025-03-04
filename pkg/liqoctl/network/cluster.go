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

package network

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	gwforge "github.com/liqotech/liqo/pkg/gateway/forge"
	enutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/utils"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	networkingutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/utils"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/maps"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// Cluster contains the information about a cluster.
type Cluster struct {
	local  *factory.Factory
	remote *factory.Factory
	waiter *wait.Waiter

	localNetworkNamespace  string
	remoteNetworkNamespace string

	localNamespaceManager  tenantnamespace.Manager
	remoteNamespaceManager tenantnamespace.Manager

	localClusterID  liqov1beta1.ClusterID
	remoteClusterID liqov1beta1.ClusterID

	networkConfiguration *networkingv1beta1.Configuration
}

// NewCluster returns a new Cluster struct.
func NewCluster(ctx context.Context, local, remote *factory.Factory, createTenantNs bool) (*Cluster, error) {
	cluster := Cluster{
		local:  local,
		remote: remote,
		waiter: wait.NewWaiterFromFactory(local),

		localNamespaceManager:  tenantnamespace.NewManager(local.KubeClient, local.CRClient.Scheme()),
		remoteNamespaceManager: tenantnamespace.NewManager(remote.KubeClient, remote.CRClient.Scheme()),
	}

	if err := cluster.SetClusterIDs(ctx); err != nil {
		return nil, err
	}

	if err := cluster.SetNamespaces(ctx, createTenantNs); err != nil {
		return nil, err
	}

	return &cluster, nil
}

// SetClusterIDs set the local and remote cluster id retrieving it from the Liqo configmaps.
func (c *Cluster) SetClusterIDs(ctx context.Context) error {
	// Get local cluster id.
	clusterID, err := liqoutils.GetClusterIDWithControllerClient(ctx, c.local.CRClient, c.local.LiqoNamespace)
	if err != nil {
		c.local.Printer.CheckErr(fmt.Errorf("an error occurred while retrieving cluster id: %v", output.PrettyErr(err)))
		return err
	}
	c.localClusterID = clusterID

	// Get remote cluster id.
	clusterID, err = liqoutils.GetClusterIDWithControllerClient(ctx, c.remote.CRClient, c.remote.LiqoNamespace)
	if err != nil {
		c.remote.Printer.CheckErr(fmt.Errorf("an error occurred while retrieving cluster id: %v", output.PrettyErr(err)))
		return err
	}
	c.remoteClusterID = clusterID

	return nil
}

// SetNamespaces sets the local and remote namespaces to the liqo-tenants namespaces (creating them if specified),
// unless the user has explicitly set custom namespaces with the `--namespace` and/or `--remote-namespace` flags.
// All the external network resources will be created in these namespaces in their respective clusters.
func (c *Cluster) SetNamespaces(ctx context.Context, createTenantNs bool) error {
	var err error

	if c.localClusterID == "" || c.remoteClusterID == "" {
		if err := c.SetClusterIDs(ctx); err != nil {
			return err
		}
	}

	switch c.local.Namespace {
	case "", corev1.NamespaceDefault:
		// If the local namespace is not set or it is the default one, create or retrieve the tenant namespace and
		// set it as the local network namespace.
		var localTenantNs *corev1.Namespace

		if createTenantNs {
			localTenantNs, err = c.localNamespaceManager.CreateNamespace(ctx, c.remoteClusterID)
			if err != nil {
				c.local.Printer.CheckErr(fmt.Errorf("an error occurred while creating local tenant namespace: %v", output.PrettyErr(err)))
				return err
			}
		} else {
			localTenantNs, err = c.localNamespaceManager.GetNamespace(ctx, c.remoteClusterID)
			if err != nil {
				c.local.Printer.CheckErr(fmt.Errorf("an error occurred while retrieving local tenant namespace: %v", output.PrettyErr(err)))
				return err
			}
		}

		c.localNetworkNamespace = localTenantNs.Name
	default:
		c.localNetworkNamespace = c.local.Namespace
	}

	switch c.remote.Namespace {
	case "", corev1.NamespaceDefault:
		// If the remote namespace is not set or it is the default one, create or retrieve the tenant namespace and
		// set it as the remote network namespace.
		var remoteTenantNs *corev1.Namespace
		creationError := false
		if createTenantNs {
			remoteTenantNs, err = c.remoteNamespaceManager.CreateNamespace(ctx, c.localClusterID)
			switch {
			case apierrors.IsForbidden(err):
				c.remote.Printer.Warning.Printfln(
					"Current user has no permissions to create a namespace in the remote cluster. " +
						"Checking whether a tenant namespace has been created in advance",
				)
				creationError = true
			case err != nil:
				c.remote.Printer.CheckErr(fmt.Errorf("an error occurred while creating remote tenant namespace: %v", output.PrettyErr(err)))
				return err
			}
		}

		if !createTenantNs || creationError {
			remoteTenantNs, err = c.remoteNamespaceManager.GetNamespace(ctx, c.localClusterID)
			if err != nil {
				c.remote.Printer.CheckErr(fmt.Errorf("an error occurred while retrieving remote tenant namespace: %v", output.PrettyErr(err)))
				return err
			}
		}

		c.remoteNetworkNamespace = remoteTenantNs.Name
	default:
		c.remoteNetworkNamespace = c.remote.Namespace
	}

	return nil
}

// SetLocalConfiguration forges and set a local Configuration to be applied on remote clusters.
func (c *Cluster) SetLocalConfiguration(ctx context.Context) error {
	// Get network configuration.
	s := c.local.Printer.StartSpinner("Retrieving network configuration")
	conf, err := forge.ConfigurationForRemoteCluster(ctx, c.local.CRClient, c.localNetworkNamespace, c.local.LiqoNamespace)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while retrieving network configuration: %v", output.PrettyErr(err)))
		return err
	}
	c.networkConfiguration = conf
	s.Success("Network configuration correctly retrieved")

	return nil
}

// SetupConfiguration sets up the network configuration.
func (c *Cluster) SetupConfiguration(ctx context.Context, conf *networkingv1beta1.Configuration) error {
	s := c.local.Printer.StartSpinner("Setting up network configuration")
	conf.Namespace = c.localNetworkNamespace
	confCopy := conf.DeepCopy()
	_, err := resource.CreateOrUpdate(ctx, c.local.CRClient, conf, func() error {
		if conf.Labels == nil {
			conf.Labels = make(map[string]string)
		}
		if confCopy.Labels != nil {
			if cID, ok := confCopy.Labels[consts.RemoteClusterID]; ok {
				conf.Labels[consts.RemoteClusterID] = cID
			}
		}
		conf.Spec.Remote = confCopy.Spec.Remote
		return nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while setting up network configuration: %v", output.PrettyErr(err)))
		return err
	}

	s.Success("Network configuration correctly set up")
	return nil
}

// CheckTemplateGwClient checks if the GatewayClient template is correctly set up.
func (c *Cluster) CheckTemplateGwClient(ctx context.Context, opts *Options) error {
	templateName := opts.ClientTemplateName
	templateNamespace := opts.ClientTemplateNamespace
	templateGVR := opts.ClientGatewayType

	s := c.local.Printer.StartSpinner(fmt.Sprintf("Checking gateway client template \"%s/%s\"",
		templateName, templateNamespace))

	_, err := c.checkTemplate(ctx, templateName, templateNamespace, templateGVR)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while checking gateway client template \"%s/%s\": %v",
			templateName, templateNamespace, output.PrettyErr(err)))
		return err
	}

	s.Success(fmt.Sprintf("Gateway client template \"%s/%s\" correctly checked",
		templateName, templateNamespace))
	return nil
}

// CheckTemplateGwServer checks if the GatewayServer template is correctly set up.
func (c *Cluster) CheckTemplateGwServer(ctx context.Context, opts *Options) error {
	templateName := opts.ServerTemplateName
	templateNamespace := opts.ServerTemplateNamespace
	templateGVR := opts.ServerGatewayType

	s := c.local.Printer.StartSpinner(fmt.Sprintf("Checking gateway server template \"%s/%s\"",
		templateName, templateNamespace))

	template, err := c.checkTemplate(ctx, templateName, templateNamespace, templateGVR)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while checking gateway server template \"%s/%s\": %v",
			templateName, templateNamespace, output.PrettyErr(err)))
		return err
	}

	if err := c.checkTemplateServerService(template, opts); err != nil {
		s.Fail(fmt.Sprintf("An error occurred while checking gateway server template \"%s/%s\": %v",
			templateName, templateNamespace, output.PrettyErr(err)))
		return err
	}

	s.Success(fmt.Sprintf("Gateway server template \"%s/%s\" correctly checked",
		templateName, templateNamespace))
	return nil
}

func (c *Cluster) checkTemplate(ctx context.Context, templateName, templateNamespace, templateGvr string) (*unstructured.Unstructured, error) {
	// Server Template Reference
	gvr, err := enutils.ParseGroupVersionResource(templateGvr)
	if err != nil {
		return nil, err
	}

	template, err := c.local.DynClient.Resource(gvr).Namespace(templateNamespace).Get(ctx, templateName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return template, nil
}

// checkTemplateServerService checks if the GatewayServer service template is correctly set up.
func (c *Cluster) checkTemplateServerService(template *unstructured.Unstructured, opts *Options) error {
	switch corev1.ServiceType(opts.ServerServiceType.Value) {
	case corev1.ServiceTypeClusterIP:
		return c.checkTemplateServerServiceClusterIP(template, opts)
	case corev1.ServiceTypeNodePort:
		return c.checkTemplateServerServiceNodePort(template, opts)
	case corev1.ServiceTypeLoadBalancer:
		return c.checkTemplateServerServiceLoadBalancer(template, opts)
	case corev1.ServiceTypeExternalName:
		return fmt.Errorf("externalName service type not supported")
	}
	return nil
}

func (c *Cluster) checkTemplateServerServiceClusterIP(_ *unstructured.Unstructured, _ *Options) error {
	return nil
}
func (c *Cluster) checkTemplateServerServiceNodePort(template *unstructured.Unstructured, opts *Options) error {
	if opts.ServerServiceNodePort == 0 {
		return nil
	}

	path := "spec.template.spec.service.spec.ports"
	templateServicePorts, err := maps.GetNestedField(template.Object, path)
	if err != nil {
		return fmt.Errorf("unable to get %s of the server template", path)
	}

	templateServicePortsSlice, ok := templateServicePorts.([]interface{})
	if !ok {
		return fmt.Errorf("unable to cast %s to []interface{}", path)
	}

	port, ok := templateServicePortsSlice[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to cast %s to map[string]interface{}", path)
	}

	_, err = maps.GetNestedField(port, "nodePort")
	if err != nil {
		// If the field is missing, it might be optional (represented by leading ?). If not, raise an error
		if _, errOptional := maps.GetNestedField(port, "?nodePort"); errOptional != nil {
			return fmt.Errorf("unable to get spec.template.spec.service.spec.ports[0].nodePort int the server template, " +
				"since you specified the flag \"--gw-server-service-nodeport\" you need to add the \"nodePort\" field in the template")
		}
	}

	return nil
}
func (c *Cluster) checkTemplateServerServiceLoadBalancer(template *unstructured.Unstructured, opts *Options) error {
	if opts.ServerServiceLoadBalancerIP == "" {
		return nil
	}

	servicePath := "spec.template.spec.service.spec"
	_, err := maps.GetNestedField(template.Object, fmt.Sprintf("%s.loadBalancerIP", servicePath))
	if err != nil {
		// If the field is missing, it might be optional (represented by leading ?). If not, raise an error
		if _, errOptional := maps.GetNestedField(template.Object, fmt.Sprintf("%s.?loadBalancerIP", servicePath)); errOptional != nil {
			return fmt.Errorf("unable to get %s of the server template, "+
				"since you specified the flag \"--gw-server-service-loadbalancerip\" you need to add the \"loadBalancerIP\" field in the template", servicePath)
		}
	}
	return nil
}

// CheckNetworkInitialized checks if the network is initialized correctly in the current cluster.
func (c *Cluster) CheckNetworkInitialized(ctx context.Context, remoteClusterID liqov1beta1.ClusterID) error {
	s := c.local.Printer.StartSpinner("Checking network is initialized correctly")

	// Get the network Configuration.
	conf, err := getters.GetConfigurationByClusterID(ctx, c.local.CRClient, remoteClusterID, c.localNetworkNamespace)
	switch {
	case client.IgnoreNotFound(err) != nil:
		s.Fail(fmt.Sprintf("An error occurred while checking network Configuration: %v", output.PrettyErr(err)))
		return err
	case apierrors.IsNotFound(err):
		s.Fail(fmt.Sprintf("Network Configuration not found. Retry to issue `liqoctl network connect`. If the issue persist, "+
			"you can try to reset the network with `liqoctl network reset`: %v", output.PrettyErr(err)))
		return err
	}

	if !networkingutils.IsConfigurationStatusSet(conf.Status) {
		err := fmt.Errorf("network Configuration status is not set yet. Retry later. If the issue persist, " +
			"you can try to reset the network with `liqoctl network reset`")
		s.Fail(err)
		return err
	}

	s.Success("Network correctly initialized")
	return nil
}

func endpointHasChanged(endpoint *networkingv1beta1.Endpoint, service *corev1.Service) bool {
	if endpoint.ServiceType != service.Spec.Type {
		return true
	}
	if len(service.Spec.Ports) > 0 {
		if endpoint.Port != service.Spec.Ports[0].Port {
			return true
		}

		if endpoint.NodePort != nil && *endpoint.NodePort != service.Spec.Ports[0].NodePort {
			return true
		}
	}
	if endpoint.LoadBalancerIP != nil && *endpoint.LoadBalancerIP != service.Spec.LoadBalancerIP ||
		endpoint.LoadBalancerIP == nil && service.Spec.LoadBalancerIP != "" {
		return true
	}
	return false
}

// EnsureGatewayServer create or updates a GatewayServer.
func (c *Cluster) EnsureGatewayServer(ctx context.Context, opts *forge.GwServerOptions) (*networkingv1beta1.GatewayServer, error) {
	s := c.local.Printer.StartSpinner("Setting up gateway server")

	// Check if the GatewayServer already exists.
	var name *string
	gwServer, err := getters.GetGatewayServerByClusterID(ctx, c.local.CRClient, c.remoteClusterID, c.localNetworkNamespace)
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	} else if err == nil {
		name = &gwServer.Name // if the GatewayServer already exists, keep its name
	}

	// Forge GatewayServer.
	gwServer, err = forge.GatewayServer(c.localNetworkNamespace, name, opts)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while forging gateway server: %v", output.PrettyErr(err)))
		return nil, err
	}

	// If the forged server endpoint has different parameters from the existing server service (if present),
	// we delete the existing gateway server so that the client can correctly connect to the new endpoint.
	var service corev1.Service
	svcNsName := types.NamespacedName{Namespace: gwServer.Namespace, Name: gwforge.GatewayResourceName(gwServer.Name)}
	err = c.local.CRClient.Get(ctx, svcNsName, &service)
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("An error occurred while retrieving gateway server service: %v", output.PrettyErr(err)))
		return nil, err
	} else if err == nil {
		// Server service already exists. Check if endpoint has changed parameters.
		if endpointHasChanged(&gwServer.Spec.Endpoint, &service) {
			if err := c.local.CRClient.Delete(ctx, gwServer); err != nil {
				s.Fail(fmt.Sprintf("An error occurred while deleting gateway server: %v", output.PrettyErr(err)))
				return nil, err
			}
			s.Success("Deleted existing gateway server")
		}
	}

	_, err = resource.CreateOrUpdate(ctx, c.local.CRClient, gwServer, func() error {
		return forge.MutateGatewayServer(gwServer, opts)
	})
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while setting up gateway server: %v", output.PrettyErr(err)))
		return nil, err
	}

	s.Success("Gateway server correctly set up")
	return gwServer, nil
}

// EnsureGatewayClient create or updates a GatewayClient.
func (c *Cluster) EnsureGatewayClient(ctx context.Context, opts *forge.GwClientOptions) (*networkingv1beta1.GatewayClient, error) {
	s := c.local.Printer.StartSpinner("Setting up gateway client")

	// Check if the GatewayClient already exists.
	var name *string
	gwClient, err := getters.GetGatewayClientByClusterID(ctx, c.local.CRClient, c.remoteClusterID, c.localNetworkNamespace)
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	} else if err == nil {
		name = &gwClient.Name // if the GatewayClient already exists, keep its name
	}

	gwClient, err = forge.GatewayClient(c.localNetworkNamespace, name, opts)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while forging gateway client: %v", output.PrettyErr(err)))
		return nil, err
	}
	_, err = resource.CreateOrUpdate(ctx, c.local.CRClient, gwClient, func() error {
		return forge.MutateGatewayClient(gwClient, opts)
	})
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while setting up gateway client: %v", output.PrettyErr(err)))
		return nil, err
	}

	s.Success("Gateway client correctly set up")
	return gwClient, nil
}

// EnsurePublicKey create or updates a PublicKey.
func (c *Cluster) EnsurePublicKey(ctx context.Context, remoteClusterID liqov1beta1.ClusterID,
	key []byte, ownerGateway metav1.Object) error {
	s := c.local.Printer.StartSpinner("Creating public key")

	// Check if the PublicKey already exists.
	var name *string
	pk, err := getters.GetPublicKeyByClusterID(ctx, c.local.CRClient, remoteClusterID, c.localNetworkNamespace)
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("An error occurred while retrieving public key: %v", output.PrettyErr(err)))
		return err
	} else if err == nil {
		name = &pk.Name // if the PublicKey already exists, keep its name
	}

	pubKey, err := forge.PublicKey(c.localNetworkNamespace, name, remoteClusterID, key)
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while forging public key: %v", output.PrettyErr(err)))
		return err
	}
	_, err = resource.CreateOrUpdate(ctx, c.local.CRClient, pubKey, func() error {
		if err := forge.MutatePublicKey(pubKey, remoteClusterID, key); err != nil {
			return err
		}
		return controllerutil.SetOwnerReference(ownerGateway, pubKey, c.local.CRClient.Scheme())
	})
	if err != nil {
		s.Fail(fmt.Sprintf("An error occurred while creating public key: %v", output.PrettyErr(err)))
		return err
	}

	s.Success("Public key correctly created")
	return nil
}

// DeleteConfiguration deletes a Configuration.
// If tenantNamespace is empty this function searches in all the namespaces in the cluster.
func (c *Cluster) DeleteConfiguration(ctx context.Context, remoteClusterID liqov1beta1.ClusterID, namespace string) error {
	s := c.local.Printer.StartSpinner("Deleting network configuration")

	// Retrieve Configuration.
	conf, err := getters.GetConfigurationByClusterID(ctx, c.local.CRClient, remoteClusterID, namespace)
	if client.IgnoreNotFound(err) != nil {
		s.Fail("An error occurred while retrieving network configuration: ", output.PrettyErr(err))
		return err
	} else if apierrors.IsNotFound(err) {
		s.Success("Network configuration already deleted")
		return nil
	}

	// Delete Configuration.
	err = c.local.CRClient.Delete(ctx, conf)
	switch {
	case client.IgnoreNotFound(err) != nil:
		s.Fail("An error occurred while deleting network configuration: ", output.PrettyErr(err))
		return err
	case apierrors.IsNotFound(err):
		s.Success("Network configuration already deleted")
	default:
		s.Success("Network configuration correctly deleted")
	}

	return nil
}

// DeleteGatewayServer deletes a GatewayServer.
func (c *Cluster) DeleteGatewayServer(ctx context.Context, remoteClusterID liqov1beta1.ClusterID) error {
	s := c.local.Printer.StartSpinner("Deleting gateway server")

	// Retrieve GatewayServer.
	gwServer, err := getters.GetGatewayServerByClusterID(ctx, c.local.CRClient, remoteClusterID, c.localNetworkNamespace)
	if client.IgnoreNotFound(err) != nil {
		s.Fail("An error occurred while retrieving gateway server: ", output.PrettyErr(err))
		return err
	} else if apierrors.IsNotFound(err) {
		s.Success("Gateway server already deleted")
		return nil
	}

	// Delete GatewayServer.
	err = c.local.CRClient.Delete(ctx, gwServer)
	switch {
	case client.IgnoreNotFound(err) != nil:
		s.Fail("An error occurred while deleting gateway server: ", output.PrettyErr(err))
		return err
	case apierrors.IsNotFound(err):
		s.Success("Gateway server already deleted")
	default:
		s.Success("Gateway server correctly deleted")
	}

	return nil
}

// DeleteGatewayClient deletes a GatewayClient.
func (c *Cluster) DeleteGatewayClient(ctx context.Context, remoteClusterID liqov1beta1.ClusterID) error {
	s := c.local.Printer.StartSpinner("Deleting gateway client")

	// Retrieve GatewayClient.
	gwClient, err := getters.GetGatewayClientByClusterID(ctx, c.local.CRClient, remoteClusterID, c.localNetworkNamespace)
	if client.IgnoreNotFound(err) != nil {
		s.Fail("An error occurred while retrieving gateway client: ", output.PrettyErr(err))
		return err
	} else if apierrors.IsNotFound(err) {
		s.Success("Gateway client already deleted")
		return nil
	}

	// Delete GatewayClient.
	err = c.local.CRClient.Delete(ctx, gwClient)
	switch {
	case client.IgnoreNotFound(err) != nil:
		s.Fail("An error occurred while deleting gateway client: ", output.PrettyErr(err))
		return err
	case apierrors.IsNotFound(err):
		s.Success("Gateway client already deleted")
	default:
		s.Success("Gateway client correctly deleted")
	}

	return nil
}

// CheckAlreadyEstablishedForGwServer checks if a GatewayServer is already established.
func (c *Cluster) CheckAlreadyEstablishedForGwServer(ctx context.Context) (bool, error) {
	_, err := getters.GetGatewayServerByClusterID(ctx, c.local.CRClient, c.remoteClusterID, c.localNetworkNamespace)
	switch {
	case client.IgnoreNotFound(err) != nil:
		return false, err
	case apierrors.IsNotFound(err):
		return false, nil
	default:
		return true, nil
	}
}

// CheckAlreadyEstablishedForGwClient checks if a GatewayClient is already established.
func (c *Cluster) CheckAlreadyEstablishedForGwClient(ctx context.Context) (bool, error) {
	_, err := getters.GetGatewayClientByClusterID(ctx, c.local.CRClient, c.remoteClusterID, c.localNetworkNamespace)
	switch {
	case client.IgnoreNotFound(err) != nil:
		return false, err
	case apierrors.IsNotFound(err):
		return false, nil
	default:
		return true, nil
	}
}
