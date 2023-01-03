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

package inband

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	"github.com/liqotech/liqo/pkg/liqonet/ipam"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/authenticationtoken"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

const (
	authPort  = "https"
	proxyPort = "http"
)

// WireGuardConfig holds the WireGuard configuration.
type WireGuardConfig struct {
	PubKey       string
	EndpointIP   string
	EndpointPort string
	BackEndType  string
}

// Cluster struct that models a k8s cluster for connect and disconnect commands.
type Cluster struct {
	local  *factory.Factory
	remote *factory.Factory
	Waiter *wait.Waiter

	locTenantNamespace string
	remTenantNamespace string
	namespaceManager   tenantnamespace.Manager
	clusterID          *discoveryv1alpha1.ClusterIdentity
	foreignCluster     *discoveryv1alpha1.ForeignCluster
	netConfig          *liqogetters.NetworkConfig
	wgConfig           *WireGuardConfig
	PortForwardOpts    *PortForwardOptions
	proxyEP            *Endpoint
	authEP             *Endpoint
	authToken          string
}

// NewCluster returns a new cluster object. The cluster has to be initialized before being consumed.
func NewCluster(local, remote *factory.Factory) *Cluster {
	pfo := &PortForwardOptions{
		Namespace: local.LiqoNamespace,
		Selector:  &liqolabels.NetworkManagerPodLabelSelector,
		Config:    local.RESTConfig,
		Client:    local.CRClient,
		PortForwarder: &DefaultPortForwarder{
			genericclioptions.NewTestIOStreamsDiscard(),
		},
		RemotePort:   liqoconsts.NetworkManagerIpamPort,
		LocalPort:    0,
		Ports:        nil,
		StopChannel:  make(chan struct{}),
		ReadyChannel: make(chan struct{}),
	}

	return &Cluster{
		local:            local,
		remote:           remote,
		Waiter:           wait.NewWaiterFromFactory(local),
		namespaceManager: tenantnamespace.NewManager(local.KubeClient),
		PortForwardOpts:  pfo,
	}
}

// Init initializes the cluster struct.
func (c *Cluster) Init(ctx context.Context) error {
	// Get cluster identity.
	s := c.local.Printer.StartSpinner("retrieving cluster identity")
	selector, err := metav1.LabelSelectorAsSelector(&liqolabels.ClusterIDConfigMapLabelSelector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving cluster identity: %v", output.PrettyErr(err)))
		return err
	}
	cm, err := liqogetters.GetConfigMapByLabel(ctx, c.local.CRClient, c.local.LiqoNamespace, selector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving cluster identity: %v", output.PrettyErr(err)))
		return err
	}
	clusterID, err := liqogetters.RetrieveClusterIDFromConfigMap(cm)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving cluster identity: %v", output.PrettyErr(err)))
		return err
	}
	s.Success("cluster identity correctly retrieved")

	// Get network configuration.
	s = c.local.Printer.StartSpinner("retrieving network configuration")
	selector, err = metav1.LabelSelectorAsSelector(&liqolabels.IPAMStorageLabelSelector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving network configuration: %v", output.PrettyErr(err)))
		return err
	}
	ipamStore, err := liqogetters.GetIPAMStorageByLabel(ctx, c.local.CRClient, selector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving network configuration: %v", output.PrettyErr(err)))
		return err
	}
	netcfg, err := liqogetters.RetrieveNetworkConfiguration(ipamStore)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving network configuration: %v", output.PrettyErr(err)))
		return err
	}
	s.Success("network configuration correctly retrieved")

	// Get vpn configuration.
	s = c.local.Printer.StartSpinner("retrieving WireGuard configuration")
	selector, err = metav1.LabelSelectorAsSelector(&liqolabels.GatewayServiceLabelSelector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving WireGuard configuration: %v", output.PrettyErr(err)))
		return err
	}
	svc, err := liqogetters.GetServiceByLabel(ctx, c.local.CRClient, c.local.LiqoNamespace, selector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving WireGuard configuration: %v", output.PrettyErr(err)))
		return err
	}
	ip, port, err := liqogetters.RetrieveWGEPFromService(svc, liqoconsts.GatewayServiceAnnotationKey, liqoconsts.DriverName)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving WireGuard configuration: %v", output.PrettyErr(err)))
		return err
	}
	// If we can't parse the ip address it could be that is not in the dot-decimal notation.
	// The WireGuard IP address could be expressed using a fqdn address. So we do not check if the parsing was successful or not.
	wgIP := net.ParseIP(ip)
	selector, err = metav1.LabelSelectorAsSelector(&liqolabels.WireGuardSecretLabelSelector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving WireGuard configuration: %v", output.PrettyErr(err)))
		return err
	}
	secret, err := liqogetters.GetSecretByLabel(ctx, c.local.CRClient, c.local.LiqoNamespace, selector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving WireGuard configuration: %v", output.PrettyErr(err)))
		return err
	}
	pubKey, err := liqogetters.RetrieveWGPubKeyFromSecret(secret, liqoconsts.PublicKey)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving WireGuard configuration: %v", output.PrettyErr(err)))
		return err
	}
	// It is safe to do this even when the wgIP is nil thanks to the short circuit evaluation.
	// wgIP will be dereferenced only when it is not nil.
	if wgIP != nil && wgIP.IsPrivate() {
		s.Warning(fmt.Sprintf("wireGuard configuration correctly retrieved: endpoint IP %q seems to be private.", ip))
	} else {
		s.Success("wireGuard configuration correctly retrieved")
	}

	// Get authentication token.
	s = c.local.Printer.StartSpinner("retrieving authentication token")
	authToken, err := auth.GetToken(ctx, c.local.CRClient, c.local.LiqoNamespace)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving auth token: %v", output.PrettyErr(err)))
		return err
	}
	s.Success("authentication token correctly retrieved")

	// Get authentication endpoint.
	s = c.local.Printer.StartSpinner("retrieving authentication  endpoint")
	selector, err = metav1.LabelSelectorAsSelector(&liqolabels.AuthServiceLabelSelector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving authentication endpoint: %v", output.PrettyErr(err)))
		return err
	}
	svc, err = liqogetters.GetServiceByLabel(ctx, c.local.CRClient, c.local.LiqoNamespace, selector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving authentication endpoint: %v", output.PrettyErr(err)))
		return err
	}
	ipAuth, portAuth, err := liqogetters.RetrieveEndpointFromService(svc, corev1.ServiceTypeClusterIP, authPort)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving authentication endpoint: %v", output.PrettyErr(err)))
		return err
	}
	s.Success("authentication endpoint correctly retrieved")

	// Get proxy endpoint.
	s = c.local.Printer.StartSpinner("retrieving proxy endpoint")
	selector, err = metav1.LabelSelectorAsSelector(&liqolabels.ProxyServiceLabelSelector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving proxy endpoint: %v", output.PrettyErr(err)))
		return err
	}
	svc, err = liqogetters.GetServiceByLabel(ctx, c.local.CRClient, c.local.LiqoNamespace, selector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving proxy endpoint: %v", output.PrettyErr(err)))
		return err
	}
	ipProxy, portProxy, err := liqogetters.RetrieveEndpointFromService(svc, corev1.ServiceTypeClusterIP, proxyPort)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving proxy endpoint: %v", output.PrettyErr(err)))
		return err
	}
	s.Success("proxy endpoint correctly retrieved")

	// Set configuration
	c.clusterID = clusterID
	c.netConfig = netcfg

	c.wgConfig = &WireGuardConfig{
		PubKey:       pubKey.String(),
		EndpointIP:   ip,
		EndpointPort: port,
		BackEndType:  liqoconsts.DriverName,
	}

	c.authToken = authToken

	c.authEP = &Endpoint{
		ip:   ipAuth,
		port: portAuth,
	}

	c.proxyEP = &Endpoint{
		ip:   ipProxy,
		port: portProxy,
	}

	return nil
}

// GetClusterID returns the cluster identity.
func (c *Cluster) GetClusterID() *discoveryv1alpha1.ClusterIdentity {
	return c.clusterID
}

// GetLocTenantNS returns the tenant namespace created for the remote cluster.
func (c *Cluster) GetLocTenantNS() string {
	return c.locTenantNamespace
}

// SetRemTenantNS sets the tenant namespace of the local cluster created by the remote cluster.
func (c *Cluster) SetRemTenantNS(remTenantNamespace string) {
	c.remTenantNamespace = remTenantNamespace
}

// GetAuthToken returns the authentication token of the local cluster.
func (c *Cluster) GetAuthToken() string {
	return c.authToken
}

// GetAuthURL returns the authentication URL of the local cluster.
func (c *Cluster) GetAuthURL() string {
	return c.authEP.GetHTTPSURL()
}

// GetProxyURL returns the proxy URL of the local cluster.
func (c *Cluster) GetProxyURL() string {
	return c.proxyEP.GetHTTPURL()
}

// SetUpTenantNamespace creates the tenant namespace in the local custer for the given remote cluster.
func (c *Cluster) SetUpTenantNamespace(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	s := c.local.Printer.StartSpinner(fmt.Sprintf("creating tenant namespace for remote cluster %q", remoteClusterID.ClusterName))
	ns, err := c.namespaceManager.CreateNamespace(ctx, *remoteClusterID)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while creating tenant namespace for remote cluster %q: %v", remoteClusterID.ClusterName, err))
		return err
	}
	s.Success(fmt.Sprintf("tenant namespace %q created for remote cluster %q", ns.Name, remoteClusterID.ClusterName))
	c.locTenantNamespace = ns.Name
	return nil
}

// TearDownTenantNamespace deletes the tenant namespace in the local cluster for the given remote cluster.
func (c *Cluster) TearDownTenantNamespace(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	remName := remoteClusterID.ClusterName
	c.locTenantNamespace = tenantnamespace.GetNameForNamespace(*remoteClusterID)
	s := c.local.Printer.StartSpinner(fmt.Sprintf("removing tenant namespace %q for remote cluster %q", c.locTenantNamespace, remName))
	_, err := c.local.KubeClient.CoreV1().Namespaces().Get(ctx, c.locTenantNamespace, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("an error occurred while getting tenant namespace for remote cluster %q: %v", remName, err))
		return err
	}

	if err != nil {
		s.Warning(fmt.Sprintf("tenant namespace %q for remote cluster %q not found", c.locTenantNamespace, remName))
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			msg := fmt.Sprintf("timeout expired while waiting for the tenant namespace %q to be deleted", c.locTenantNamespace)
			s.Fail(msg)
			return errors.New(msg)
		default:
			err := c.local.KubeClient.CoreV1().Namespaces().Delete(ctx, c.locTenantNamespace, metav1.DeleteOptions{})
			if client.IgnoreNotFound(err) != nil {
				s.Fail(fmt.Sprintf("an error occurred while deleting tenant namespace %q for remote cluster %q: %v", c.locTenantNamespace, remName, err))
				return err
			}

			if err != nil {
				s.Success(fmt.Sprintf("tenant namespace %q correctly removed for remote cluster %q", c.locTenantNamespace, remName))
				return nil
			}

			time.Sleep(2 * time.Second)
		}
	}
}

// ExchangeNetworkCfg creates the local networkconfigs resource for the remote cluster, replicates it
// into the remote cluster, waits for the remote cluster to populate the status of the resource and then
// sets the remote status in the local networkconfigs resource.
func (c *Cluster) ExchangeNetworkCfg(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	// Enforce the network configuration in the local cluster.
	s := c.local.Printer.StartSpinner("creating network configuration in local cluster")
	if err := c.enforceNetworkCfg(ctx, remoteClusterID, true); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while creating network configuration in local cluster: %v", output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("network configuration created in local cluster %q", c.clusterID.ClusterName))

	// Enforce the network configuration in the local cluster.
	s = c.local.Printer.StartSpinner(fmt.Sprintf("creating network configuration in remote cluster %q", remoteClusterID.ClusterName))
	if err := c.enforceNetworkCfg(ctx, remoteClusterID, false); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while creating network configuration in remote cluster: %v", output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("network configuration created in remote cluster %q", remoteClusterID.ClusterName))

	// Wait for the network configuration to be processed by the remote cluster.
	s = c.local.Printer.StartSpinner(fmt.Sprintf("waiting network configuration to be processed by remote cluster %q", remoteClusterID.ClusterName))
	netcfg, err := c.waitForNetCfg(ctx, remoteClusterID, 60*time.Second)
	if err != nil {
		s.Fail(err)
		return err
	}
	s.UpdateText(fmt.Sprintf("reflecting network configuration status from cluster %q", remoteClusterID.ClusterName))
	if err := c.reflectNetworkCfgStatus(ctx, remoteClusterID, &netcfg.Status); err != nil {
		s.Fail(err)
		return err
	}
	s.Success(fmt.Sprintf("network configuration status correctly reflected from cluster %q", remoteClusterID.ClusterName))
	return nil
}

// enforceNetworkCfg enforces the presence of the networkconfigs resource for a given remote cluster.
func (c *Cluster) enforceNetworkCfg(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity, local bool) error {
	// Get the network config.
	netcfg, err := c.getNetworkCfg(ctx, remoteClusterID, local)
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	if err != nil {
		return c.createNetworkCfg(ctx, remoteClusterID, local)
	}

	return c.updateNetworkCfgSpec(ctx, netcfg, remoteClusterID, local)
}

// getNetworkCfg retrieves the networkconfigs resource for a given remote cluster.
func (c *Cluster) getNetworkCfg(ctx context.Context,
	remoteClusterID *discoveryv1alpha1.ClusterIdentity, local bool) (*netv1alpha1.NetworkConfig, error) {
	var cl client.Client
	var selector labels.Selector
	var err error
	var ns string

	if local {
		cl = c.local.CRClient
		if selector, err = labelSelectorForReplicatedResource(remoteClusterID, local); err != nil {
			return nil, err
		}
		ns = c.locTenantNamespace
	} else {
		cl = c.remote.CRClient
		if selector, err = labelSelectorForReplicatedResource(c.clusterID, local); err != nil {
			return nil, err
		}
		ns = c.remTenantNamespace
	}

	// Get the network config.
	return liqogetters.GetNetworkConfigByLabel(ctx, cl, ns, selector)
}

// createNetworkCfg creates the networkconfigs resource for the given remote cluster.
func (c *Cluster) createNetworkCfg(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity, local bool) error {
	netcfg := &netv1alpha1.NetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fcutils.UniqueName(remoteClusterID),
			Namespace: c.locTenantNamespace,
			Labels: map[string]string{
				liqoconsts.ReplicationRequestedLabel:   strconv.FormatBool(true),
				liqoconsts.ReplicationDestinationLabel: remoteClusterID.ClusterID,
			},
		},
	}
	c.populateNetworkCfg(netcfg, remoteClusterID, local)

	if local {
		return c.local.CRClient.Create(ctx, netcfg)
	}

	return c.remote.CRClient.Create(ctx, netcfg)
}

// updateNetworkCfgSpec given the local networkconfigs resource the spec is reflected on the
// remote instance.
func (c *Cluster) updateNetworkCfgSpec(ctx context.Context, netcfg *netv1alpha1.NetworkConfig,
	remoteClusterID *discoveryv1alpha1.ClusterIdentity, local bool) error {
	original := netcfg.DeepCopy()

	c.populateNetworkCfg(netcfg, remoteClusterID, local)

	if reflect.DeepEqual(original, netcfg) {
		return nil
	}

	if local {
		return c.local.CRClient.Update(ctx, netcfg)
	}

	return c.remote.CRClient.Update(ctx, netcfg)
}

// updateNetworkCfgStatus given the remote networkconfigs resource the status is reflected on the
// local instance.
func (c *Cluster) updateNetworkCfgStatus(ctx context.Context, netcfg *netv1alpha1.NetworkConfig,
	newStatus *netv1alpha1.NetworkConfigStatus) error {
	if reflect.DeepEqual(netcfg.Status, newStatus) {
		return nil
	}

	netcfg.Status = *newStatus
	return c.local.CRClient.Status().Update(ctx, netcfg)
}

// reflectNetworkCfgStatus given the remote networkconfigs resource the status is reflected on the
// local instance.
func (c *Cluster) reflectNetworkCfgStatus(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity,
	status *netv1alpha1.NetworkConfigStatus) error {
	// Get the local networkconfig
	netcfg, err := c.getNetworkCfg(ctx, remoteClusterID, true)
	if err != nil {
		return nil
	}

	return c.updateNetworkCfgStatus(ctx, netcfg, status)
}

// waitForNetCfg wait until a remote networkconfigs resource is processed by the remote cluster or the timeout expires.
func (c *Cluster) waitForNetCfg(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity,
	timeout time.Duration) (*netv1alpha1.NetworkConfig, error) {
	deadLine := time.After(timeout)
	for {
		select {
		case <-deadLine:
			return nil, fmt.Errorf("timeout (%.0fs) expired while waiting for cluster %q to process the networkconfig",
				timeout.Seconds(), remoteClusterID.ClusterName)
		default:
			netcfg, err := c.getNetworkCfg(ctx, remoteClusterID, false)
			if err != nil {
				return nil, err
			}
			if netcfg.Status.Processed {
				return netcfg, err
			}
			time.Sleep(2 * time.Second)
		}
	}
}

// populateNetworkCfg sets the fields of a given networkconfigs resource for a remote cluster. Flag is used to
// denote if the resource is local or remote.
func (c *Cluster) populateNetworkCfg(netcfg *netv1alpha1.NetworkConfig, remoteClusterID *discoveryv1alpha1.ClusterIdentity, local bool) {
	clusterID := remoteClusterID.ClusterID

	if netcfg.Labels == nil {
		netcfg.Labels = map[string]string{}
	}

	netcfg.Labels[liqoconsts.ReplicationRequestedLabel] = strconv.FormatBool(true)
	netcfg.Labels[liqoconsts.ReplicationDestinationLabel] = clusterID

	if !local {
		// setting the right namespace in the remote cluster
		netcfg.Namespace = c.remTenantNamespace
		// setting the replication label to false
		netcfg.Labels[liqoconsts.ReplicationRequestedLabel] = strconv.FormatBool(false)
		// setting replication status to true
		netcfg.Labels[liqoconsts.ReplicationStatusLabel] = strconv.FormatBool(true)
		// setting originID i.e clusterID of home cluster
		netcfg.Labels[liqoconsts.ReplicationOriginLabel] = c.clusterID.ClusterID
	}

	netcfg.Spec.RemoteCluster = *remoteClusterID
	netcfg.Spec.PodCIDR = c.netConfig.PodCIDR
	netcfg.Spec.ExternalCIDR = c.netConfig.ExternalCIDR
	netcfg.Spec.EndpointIP = c.wgConfig.EndpointIP
	netcfg.Spec.BackendType = liqoconsts.DriverName

	if netcfg.Spec.BackendConfig == nil {
		netcfg.Spec.BackendConfig = map[string]string{}
	}

	netcfg.Spec.BackendConfig[liqoconsts.PublicKey] = c.wgConfig.PubKey
	netcfg.Spec.BackendConfig[liqoconsts.ListeningPort] = c.wgConfig.EndpointPort
}

// labelSelectorForReplicatedResource returns the correct labels used to get a replicated resource.
// Based on the local flag it returns labels for the local one or the remote one.
func labelSelectorForReplicatedResource(clusterID *discoveryv1alpha1.ClusterIdentity, local bool) (labels.Selector, error) {
	if local {
		labelsSet := labels.Set{
			liqoconsts.ReplicationDestinationLabel: clusterID.ClusterID,
			liqoconsts.ReplicationRequestedLabel:   strconv.FormatBool(true),
		}

		return labels.ValidatedSelectorFromSet(labelsSet)
	}
	labelsSet := labels.Set{
		liqoconsts.ReplicationOriginLabel: clusterID.ClusterID,
		liqoconsts.ReplicationStatusLabel: strconv.FormatBool(true),
	}

	return labels.ValidatedSelectorFromSet(labelsSet)
}

// PortForwardIPAM starts the port forwarding for the IPAM service.
func (c *Cluster) PortForwardIPAM(ctx context.Context) error {
	s := c.local.Printer.StartSpinner("port-forwarding IPAM service")

	if err := c.PortForwardOpts.RunPortForward(ctx); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while port-forwarding IPAM service: %v", output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("IPAM service correctly port-forwarded %q", c.PortForwardOpts.Ports[0]))

	return nil
}

// StopPortForwardIPAM stops the port forwarding for the IPAM service.
func (c *Cluster) StopPortForwardIPAM() {
	s := c.local.Printer.StartSpinner("stopping IPAM service port-forward")
	c.PortForwardOpts.StopPortForward()
	s.Success(fmt.Sprintf("IPAM service port-forward correctly stopped %q", c.PortForwardOpts.Ports[0]))
}

// MapProxyIPForCluster maps the ClusterIP address of the local proxy on the local external CIDR as seen by the remote cluster.
func (c *Cluster) MapProxyIPForCluster(ctx context.Context, ipamClient ipam.IpamClient, remoteCluster *discoveryv1alpha1.ClusterIdentity) error {
	clusterName := remoteCluster.ClusterName
	ipToBeRemapped := c.proxyEP.GetIP()

	s := c.local.Printer.StartSpinner(fmt.Sprintf("mapping proxy ip %q for cluster %q", ipToBeRemapped, clusterName))

	ip, err := mapServiceForCluster(ctx, ipamClient, ipToBeRemapped, remoteCluster)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while mapping proxy address %q for cluster %q: %v", ipToBeRemapped, clusterName, err))
		return err
	}

	c.proxyEP.SetRemappedIP(ip)

	s.Success(fmt.Sprintf("proxy address %q remapped to %q for remote cluster %q", ipToBeRemapped, ip, clusterName))

	return nil
}

// UnmapProxyIPForCluster unmaps the ClusterIP address of the local proxy on the local external CIDR as seen by the remote cluster.
func (c *Cluster) UnmapProxyIPForCluster(ctx context.Context, ipamClient ipam.IpamClient, remoteCluster *discoveryv1alpha1.ClusterIdentity) error {
	clusterName := remoteCluster.ClusterName
	ipToBeUnmapped := c.proxyEP.GetIP()

	s := c.local.Printer.StartSpinner(fmt.Sprintf("unmapping proxy ip for cluster %q", clusterName))

	if err := unmapServiceForCluster(ctx, ipamClient, ipToBeUnmapped, remoteCluster); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while unmapping proxy address %q for cluster %q: %v", ipToBeUnmapped, clusterName, err))
		return err
	}

	s.Success(fmt.Sprintf("proxy address %q unmapped for remote cluster %q", ipToBeUnmapped, clusterName))

	return nil
}

// MapAuthIPForCluster maps the ClusterIP address of the local auth service on the local external CIDR as seen by the remote cluster.
func (c *Cluster) MapAuthIPForCluster(ctx context.Context, ipamClient ipam.IpamClient, remoteCluster *discoveryv1alpha1.ClusterIdentity) error {
	clusterName := remoteCluster.ClusterName
	ipToBeRemapped := c.authEP.GetIP()

	s := c.local.Printer.StartSpinner(fmt.Sprintf("mapping auth ip %q for cluster %q", ipToBeRemapped, clusterName))

	ip, err := mapServiceForCluster(ctx, ipamClient, ipToBeRemapped, remoteCluster)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while mapping auth address %q for cluster %q: %v", ipToBeRemapped, clusterName, err))
		return err
	}

	c.authEP.SetRemappedIP(ip)

	s.Success(fmt.Sprintf("auth address %q remapped to %q for remote cluster %q", ipToBeRemapped, ip, clusterName))

	return nil
}

// UnmapAuthIPForCluster unmaps the ClusterIP address of the local auth service on the local external CIDR as seen by the remote cluster.
func (c *Cluster) UnmapAuthIPForCluster(ctx context.Context, ipamClient ipam.IpamClient, remoteCluster *discoveryv1alpha1.ClusterIdentity) error {
	clusterName := remoteCluster.ClusterName
	ipToBeUnmapped := c.authEP.GetIP()

	s := c.local.Printer.StartSpinner(fmt.Sprintf("unmapping auth ip %q for cluster %q", ipToBeUnmapped, clusterName))

	if err := unmapServiceForCluster(ctx, ipamClient, ipToBeUnmapped, remoteCluster); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while unmapping auth address %q for cluster %q: %v", ipToBeUnmapped, clusterName, err))
		return err
	}

	s.Success(fmt.Sprintf("auth address %q unmapped for remote cluster %q", ipToBeUnmapped, clusterName))

	return nil
}

// NewIPAMClient creates and returns a client to the IPAM service.
func (c *Cluster) NewIPAMClient(ctx context.Context) (ipam.IpamClient, error) {
	ipamTarget := fmt.Sprintf("%s:%d", "localhost", c.PortForwardOpts.LocalPort)

	dialctx, cancel := context.WithTimeout(ctx, 10*time.Second)

	connection, err := grpc.DialContext(dialctx, ipamTarget, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	cancel()
	if err != nil {
		c.local.Printer.Error.Printfln("an error occurred while creating IPAM client: %v", output.PrettyErr(err))
		return nil, err
	}

	return ipam.NewIpamClient(connection), nil
}

// mapServiceForCluster maps a local ip address to be consumed by a given remote cluster.
func mapServiceForCluster(ctx context.Context, ipamClient ipam.IpamClient, ipToBeRemapped string,
	remoteCluster *discoveryv1alpha1.ClusterIdentity) (string, error) {
	mapRequest := &ipam.MapRequest{
		ClusterID: remoteCluster.ClusterID,
		Ip:        ipToBeRemapped,
	}

	resp, err := ipamClient.MapEndpointIP(ctx, mapRequest)
	if err != nil {
		return "", err
	}

	return resp.Ip, nil
}

// unmapServiceForCluster releases a mapped local ip address previously mapped for given remote cluster.
func unmapServiceForCluster(ctx context.Context, ipamClient ipam.IpamClient, ipToBeUnmapped string,
	remoteCluster *discoveryv1alpha1.ClusterIdentity) error {
	var err error

	unMapRequest := &ipam.UnmapRequest{
		ClusterID: remoteCluster.ClusterID,
		Ip:        ipToBeUnmapped,
	}
	// When the natmapping CR resource does not exist an error is returned if we try to unmap an IP address.
	// In that case we just ignore the error and continue. That could happen whe we call liqoctl disconnect
	// multiple times.
	if _, err = ipamClient.UnmapEndpointIP(ctx, unMapRequest); err != nil && strings.Contains(err.Error(), "must be initialized first") {
		return nil
	}

	return err
}

// CheckForeignCluster retrieves the ForeignCluster resource associated with the remote cluster (if any), and stores it for later usage.
// Additionally, it performs the appropriate sanity checks, ensuring that the type of peering is not mutated.
func (c *Cluster) CheckForeignCluster(ctx context.Context, remoteIdentity *discoveryv1alpha1.ClusterIdentity) (err error) {
	s := c.local.Printer.StartSpinner(fmt.Sprintf("checking if the foreign cluster resource for remote cluster %q already exists", remoteIdentity))

	defer func() {
		if err != nil {
			s.Fail(err)
		}
	}()

	// Check that the cluster is not peering towards itself.
	if c.clusterID.ClusterID == remoteIdentity.ClusterID {
		return fmt.Errorf("the clusterID %q of remote cluster %q is equal to the ID of the local cluster", c.clusterID.ClusterID, remoteIdentity)
	}

	// Get existing foreign cluster if it does exist
	fc, err := fcutils.GetForeignClusterByID(ctx, c.local.CRClient, remoteIdentity.ClusterID)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("an error occurred while getting foreign cluster for remote cluster %q: %w", remoteIdentity, err)
	}

	if k8serrors.IsNotFound(err) {
		// If not found, create a new struct, which will be applied in a later step.
		c.foreignCluster = &discoveryv1alpha1.ForeignCluster{ObjectMeta: metav1.ObjectMeta{Name: remoteIdentity.ClusterName}}
		s.Success(fmt.Sprintf("foreign cluster for remote cluster %q not found: marked for creation", remoteIdentity))
		return nil
	}

	// Otherwise, check that the type of peering is not mutated.
	if fc.Spec.PeeringType != discoveryv1alpha1.PeeringTypeInBand {
		return fmt.Errorf("a peering of type %s already exists towards remote cluster %q, cannot be changed to %s",
			fc.Spec.PeeringType, remoteIdentity, discoveryv1alpha1.PeeringTypeInBand)
	}
	c.foreignCluster = fc

	s.Success(fmt.Sprintf("foreign cluster for remote cluster %q correctly retrieved", remoteIdentity))
	return nil
}

// EnforceForeignCluster enforces the presence of the foreignclusters instance for a given remote cluster.
// This function must be executed after CheckForeignCluster, which retrieves the ForeignCluster and performs the appropriate sanity checks.
// The newly created foreigncluster has the following fields set to:
//   - ForeignAuthURL -> the remapped ip address for the local cluster of the auth service living in the remote cluster;
//   - ForeignProxyURL -> the remapped ip address for the local cluster of the proxy service living in the remote cluster;
//   - NetworkingEnabled -> No, we do not want the networking to be handled by the peering process. Networking is
//     handled manually by the licoctl connect/disconnect commands.
func (c *Cluster) EnforceForeignCluster(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity,
	token, authURL, proxyURL string) error {
	remID := remoteClusterID.ClusterID
	remName := remoteClusterID.ClusterName

	s := c.local.Printer.StartSpinner(fmt.Sprintf("configuring the foreign cluster resource for the remote cluster %q", remName))
	if err := authenticationtoken.StoreInSecret(ctx, c.local.KubeClient, remID, token, c.local.LiqoNamespace); err != nil {
		msg := fmt.Sprintf("an error occurred while storing auth token for remote cluster %q: %v", remName, err)
		s.Fail(msg)
		return errors.New(msg)
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, c.local.CRClient, c.foreignCluster, func() error {
		c.foreignCluster.Spec.PeeringType = discoveryv1alpha1.PeeringTypeInBand
		c.foreignCluster.Spec.ClusterIdentity = *remoteClusterID
		c.foreignCluster.Spec.ForeignAuthURL = authURL
		c.foreignCluster.Spec.ForeignProxyURL = proxyURL

		if c.foreignCluster.Spec.OutgoingPeeringEnabled == "" {
			// If outgoing peering is currently not set, then it is set to No, to prevent possible race conditions while
			// waiting for the other foreign cluster to be created. Then, this flag will be appropriately set by the
			// EnforceOutgoingPeeringFlag function.
			c.foreignCluster.Spec.OutgoingPeeringEnabled = discoveryv1alpha1.PeeringEnabledNo
		}
		if c.foreignCluster.Spec.IncomingPeeringEnabled == "" {
			c.foreignCluster.Spec.IncomingPeeringEnabled = discoveryv1alpha1.PeeringEnabledAuto
		}
		if c.foreignCluster.Spec.InsecureSkipTLSVerify == nil {
			c.foreignCluster.Spec.InsecureSkipTLSVerify = pointer.Bool(true)
		}
		return nil
	}); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while creating/updating foreign cluster for remote cluster %q: %v", remName, err))
		return err
	}
	s.Success(fmt.Sprintf("foreign cluster for remote cluster %q correctly configured", remName))
	return nil
}

// EnforceOutgoingPeeringFlag sets the outgoing peering flag for a given foreign cluster.
func (c *Cluster) EnforceOutgoingPeeringFlag(ctx context.Context, remoteID *discoveryv1alpha1.ClusterIdentity, enabled bool) error {
	s := c.local.Printer.StartSpinner(fmt.Sprintf("configuring the outgoing peering flag for the remote cluster %q", remoteID.ClusterName))
	if _, err := controllerutil.CreateOrUpdate(ctx, c.local.CRClient, c.foreignCluster, func() error {
		if enabled {
			c.foreignCluster.Spec.OutgoingPeeringEnabled = discoveryv1alpha1.PeeringEnabledYes
		} else {
			c.foreignCluster.Spec.OutgoingPeeringEnabled = discoveryv1alpha1.PeeringEnabledNo
		}
		return nil
	}); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while configuring the outgoing peering flag for remote cluster %q: %v", remoteID.ClusterName, err))
		return err
	}
	s.Success(fmt.Sprintf("outgoing peering flag for remote cluster %q correctly configured", remoteID.ClusterName))
	return nil
}

// DeleteForeignCluster deletes the foreignclusters instance for the given remote cluster.
func (c *Cluster) DeleteForeignCluster(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	remID := remoteClusterID.ClusterID
	remName := remoteClusterID.ClusterName

	s := c.local.Printer.StartSpinner(fmt.Sprintf("deleting foreigncluster for the remote cluster %q", remName))
	if c.clusterID.ClusterID == remoteClusterID.ClusterID {
		msg := fmt.Sprintf("the clusterID %q of remote cluster %q is equal to the ID of the local cluster", remID, remName)
		s.Fail(msg)
		return errors.New(msg)
	}

	// Get existing foreign cluster if it does exist
	fc, err := fcutils.GetForeignClusterByID(ctx, c.local.CRClient, remID)
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("an error occurred while getting foreign cluster for remote cluster %q: %v", remName, err))
		return err
	}

	if k8serrors.IsNotFound(err) {
		s.Warning(fmt.Sprintf("it seems that the foreign cluster for remote cluster %q has been already removed", remName))
		return nil
	}

	if err := c.local.CRClient.Delete(ctx, fc); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while deleting foreigncluster for remote cluster %q: %v", remName, err))
		return err
	}

	s.Success(fmt.Sprintf("foreigncluster deleted for remote cluster %q", remName))
	return nil
}

// DisablePeering disables the peering for the remote cluster by patching the foreigncusters resource.
func (c *Cluster) DisablePeering(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) (err error) {
	remID := remoteClusterID.ClusterID
	remName := remoteClusterID.ClusterName
	s := c.local.Printer.StartSpinner(fmt.Sprintf("disabling peering for the remote cluster %q", remName))

	defer func() {
		if err != nil {
			s.Fail(err)
		}
	}()

	if c.clusterID.ClusterID == remoteClusterID.ClusterID {
		return fmt.Errorf("the clusterID %q of remote cluster %q is equal to the ID of the local cluster", remID, remName)
	}

	// Get existing foreign cluster if it does exist
	fc, err := fcutils.GetForeignClusterByID(ctx, c.local.CRClient, remID)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("an error occurred while getting foreign cluster for remote cluster %q: %w", remName, err)
	}

	// Not nil only if the foreign cluster does not exist.
	if err != nil {
		s.Warning(fmt.Sprintf("it seems that the foreign cluster for remote cluster %q has been already removed", remName))
		return nil
	}

	// Do not proceed if the peering is not in-band.
	if fc.Spec.PeeringType != discoveryv1alpha1.PeeringTypeInBand {
		return fmt.Errorf("the peering type towards remote cluster %q is %s, expected %s",
			remName, fc.Spec.PeeringType, discoveryv1alpha1.PeeringTypeInBand)
	}

	// Set outgoing peering to no.
	if _, err = controllerutil.CreateOrUpdate(ctx, c.local.CRClient, fc, func() error {
		fc.Spec.OutgoingPeeringEnabled = "No"
		return nil
	}); err != nil {
		return fmt.Errorf("an error occurred while disabling peering for remote cluster %q: %w", remName, err)
	}
	s.Success(fmt.Sprintf("peering correctly disabled for remote cluster %q", remName))

	return nil
}
