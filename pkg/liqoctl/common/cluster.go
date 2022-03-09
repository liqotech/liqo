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

package common

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/pterm/pterm"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/liqonet/ipam"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/authenticationtoken"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

const (
	// Cluster1Name name used in output messages for cluster passed as first argument.
	Cluster1Name = "cluster1"
	// Cluster1Color color used in output messages for cluster passed as first argument.
	Cluster1Color = pterm.FgLightBlue
	// Cluster2Name name used in output messages for cluster passed as second argument.
	Cluster2Name = "cluster2"
	// Cluster2Color color used in output messages for cluster passed as second argument.
	Cluster2Color = pterm.FgLightMagenta

	proxyName = "liqo-proxy"

	authPort = "https"
)

var (
	// Scheme used for the controller runtime clients.
	Scheme *runtime.Scheme
)

func init() {
	Scheme = runtime.NewScheme()
	utilruntime.Must(sharingv1alpha1.AddToScheme(Scheme))
	utilruntime.Must(discoveryv1alpha1.AddToScheme(Scheme))
	utilruntime.Must(netv1alpha1.AddToScheme(Scheme))
	utilruntime.Must(corev1.AddToScheme(Scheme))
}

// Cluster struct that models a k8s cluster for connect and disconnect commands.
type Cluster struct {
	locK8sClient       k8s.Interface
	locCtrlRunClient   client.Client
	remCtrlRunClient   client.Client
	locTenantNamespace string
	remTenantNamespace string
	namespaceManager   tenantnamespace.Manager
	namespace          string
	name               string
	clusterID          *discoveryv1alpha1.ClusterIdentity
	netConfig          *liqogetters.NetworkConfig
	wgConfig           *WireGuardConfig
	printer            Printer
	PortForwardOpts    *PortForwardOptions
	proxyEP            *Endpoint
	authEP             *Endpoint
	authToken          string
}

// NewPrinter creates a new printer.
func NewPrinter(name string, printerColor pterm.Color) *Printer {
	genericPrinter := pterm.PrefixPrinter{
		Prefix: pterm.Prefix{},
		Scope: pterm.Scope{
			Text:  name,
			Style: pterm.NewStyle(printerColor),
		},
		MessageStyle: pterm.NewStyle(pterm.FgDefault),
	}

	successPrinter := SuccessPrinter.WithScope(
		pterm.Scope{
			Text:  name,
			Style: pterm.NewStyle(printerColor),
		})

	warningPrinter := WarningPrinter.WithScope(
		pterm.Scope{
			Text:  name,
			Style: pterm.NewStyle(printerColor),
		})

	errorPrinter := ErrorPrinter.WithScope(
		pterm.Scope{
			Text:  name,
			Style: pterm.NewStyle(printerColor),
		})

	return &Printer{
		Info: genericPrinter.WithPrefix(pterm.Prefix{
			Text:  "[INFO]",
			Style: pterm.NewStyle(pterm.FgCyan),
		}),
		Success: successPrinter,
		Warning: warningPrinter,
		Error:   errorPrinter,
		Spinner: &pterm.SpinnerPrinter{
			Sequence:            spinnerCharset,
			Style:               pterm.NewStyle(printerColor),
			Delay:               time.Millisecond * 100,
			MessageStyle:        pterm.NewStyle(printerColor),
			SuccessPrinter:      successPrinter,
			FailPrinter:         errorPrinter,
			WarningPrinter:      warningPrinter,
			RemoveWhenDone:      false,
			ShowTimer:           true,
			TimerRoundingFactor: time.Second,
			TimerStyle:          &pterm.ThemeDefault.TimerStyle,
		},
	}
}

// NewCluster returns a new cluster object. The cluster has to be initialized before being consumed.
func NewCluster(localK8sClient k8s.Interface, localCtrlRunClient, remoteCtrlRunClient client.Client,
	restConfig *rest.Config, namespace, name string, printerColor pterm.Color) *Cluster {
	printer := NewPrinter(name, printerColor)

	pfo := &PortForwardOptions{
		Namespace: namespace,
		Selector:  &liqolabels.NetworkManagerPodLabelSelector,
		Config:    restConfig,
		Client:    localCtrlRunClient,
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
		name:             name,
		namespace:        namespace,
		printer:          *printer,
		locK8sClient:     localK8sClient,
		locCtrlRunClient: localCtrlRunClient,
		remCtrlRunClient: remoteCtrlRunClient,
		namespaceManager: tenantnamespace.NewTenantNamespaceManager(localK8sClient),
		PortForwardOpts:  pfo,
	}
}

// Init initializes the cluster struct.
func (c *Cluster) Init(ctx context.Context) error {
	// Get cluster identity.
	s, _ := c.printer.Spinner.Start("retrieving cluster identity")
	selector, err := metav1.LabelSelectorAsSelector(&liqolabels.ClusterIDConfigMapLabelSelector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving cluster identity: %v", err))
		return err
	}
	cm, err := liqogetters.GetConfigMapByLabel(ctx, c.locCtrlRunClient, c.namespace, selector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving cluster identity: %v", err))
		return err
	}
	clusterID, err := liqogetters.RetrieveClusterIDFromConfigMap(cm)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving cluster identity: %v", err))
		return err
	}
	s.Success("cluster identity correctly retrieved")

	// Get network configuration.
	s, _ = c.printer.Spinner.Start("retrieving network configuration")
	selector, err = metav1.LabelSelectorAsSelector(&liqolabels.IPAMStorageLabelSelector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving network configuration: %v", err))
		return err
	}
	ipamStore, err := liqogetters.GetIPAMStorageByLabel(ctx, c.locCtrlRunClient, "default", selector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving network configuration: %v", err))
		return err
	}
	netcfg, err := liqogetters.RetrieveNetworkConfiguration(ipamStore)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving network configuration: %v", err))
		return err
	}
	s.Success("network configuration correctly retrieved")

	// Get vpn configuration.
	s, _ = c.printer.Spinner.Start("retrieving WireGuard configuration")
	selector, err = metav1.LabelSelectorAsSelector(&liqolabels.GatewayServiceLabelSelector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving WireGuard configuration: %v", err))
		return err
	}
	svc, err := liqogetters.GetServiceByLabel(ctx, c.locCtrlRunClient, c.namespace, selector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving WireGuard configuration: %v", err))
		return err
	}
	ip, port, err := liqogetters.RetrieveWGEPFromService(svc, liqoconsts.GatewayServiceAnnotationKey, liqoconsts.DriverName)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving WireGuard configuration: %v", err))
		return err
	}
	selector, err = metav1.LabelSelectorAsSelector(&liqolabels.WireGuardSecretLabelSelector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving WireGuard configuration: %v", err))
		return err
	}
	secret, err := liqogetters.GetSecretByLabel(ctx, c.locCtrlRunClient, c.namespace, selector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving WireGuard configuration: %v", err))
		return err
	}
	pubKey, err := liqogetters.RetrieveWGPubKeyFromSecret(secret, liqoconsts.PublicKey)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving WireGuard configuration: %v", err))
		return err
	}
	s.Success("wireGuard configuration correctly retrieved")

	// Get authentication token.
	s, _ = c.printer.Spinner.Start("retrieving authentication token")
	authToken, err := auth.GetToken(ctx, c.locCtrlRunClient, c.namespace)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving auth token: %v", err))
		return err
	}
	s.Success("authentication token correctly retrieved")

	// Get authentication endpoint.
	s, _ = c.printer.Spinner.Start("retrieving authentication  endpoint")
	selector, err = metav1.LabelSelectorAsSelector(&liqolabels.AuthServiceLabelSelector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving authentication endpoint: %v", err))
		return err
	}
	svc, err = liqogetters.GetServiceByLabel(ctx, c.locCtrlRunClient, c.namespace, selector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving authentication endpoint: %v", err))
		return err
	}
	ipAuth, portAuth, err := liqogetters.RetrieveEndpointFromService(svc, corev1.ServiceTypeClusterIP, authPort)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving authentication endpoint: %v", err))
		return err
	}
	s.Success("authentication endpoint correctly retrieved")

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
	s, _ := c.printer.Spinner.Start(fmt.Sprintf("creating tenant namespace for remote cluster {%s}", remoteClusterID.ClusterName))
	ns, err := c.namespaceManager.CreateNamespace(*remoteClusterID)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while creating tenant namespace for remote cluster {%s}: %v", remoteClusterID.ClusterName, err))
		return err
	}
	s.Success(fmt.Sprintf("tenant namespace {%s} created for remote cluster {%s}", ns.Name, remoteClusterID.ClusterName))
	c.locTenantNamespace = ns.Name
	return nil
}

// TearDownTenantNamespace deletes the tenant namespace in the local cluster for the given remote cluster.
func (c *Cluster) TearDownTenantNamespace(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity, timeout time.Duration) error {
	remName := remoteClusterID.ClusterName
	c.locTenantNamespace = tenantnamespace.GetNameForNamespace(*remoteClusterID)
	s, _ := c.printer.Spinner.Start(fmt.Sprintf("removing tenant namespace {%s} for remote cluster {%s}", c.locTenantNamespace, remName))
	_, err := c.locK8sClient.CoreV1().Namespaces().Get(ctx, c.locTenantNamespace, metav1.GetOptions{})
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("an error occurred while getting tenant namespace for remote cluster {%s}: %v", remName, err))
		return err
	}

	if err != nil {
		s.Warning(fmt.Sprintf("tenant namespace {%s} for remote cluster {%s} not found", c.locTenantNamespace, remName))
		return nil
	}

	deadLine := time.After(timeout)
	for {
		select {
		case <-deadLine:
			msg := fmt.Sprintf("timout (%.0fs) expired while waiting tenant namespace {%s} to be deleted",
				timeout.Seconds(), c.locTenantNamespace)
			s.Fail(msg)
			return fmt.Errorf(msg)
		default:
			err := c.locK8sClient.CoreV1().Namespaces().Delete(ctx, c.locTenantNamespace, metav1.DeleteOptions{})
			if client.IgnoreNotFound(err) != nil {
				s.Fail(fmt.Sprintf("an error occurred while deleting tenant namespace {%s} for remote cluster {%s}: %v", c.locTenantNamespace, remName, err))
				return err
			}

			if err != nil {
				s.Success(fmt.Sprintf("tenant namespace {%s} correctly removed for remote cluster {%s}", c.locTenantNamespace, remName))
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
	s, _ := c.printer.Spinner.Start("creating network configuration in local cluster")
	if err := c.enforceNetworkCfg(ctx, remoteClusterID, true); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while creating network configuration in local cluster: %v", err))
		return err
	}
	s.Success(fmt.Sprintf("network configuration created in local cluster {%s}", c.clusterID.ClusterName))

	// Enforce the network configuration in the local cluster.
	s, _ = c.printer.Spinner.Start(fmt.Sprintf("creating network configuration in remote cluster {%s}", remoteClusterID.ClusterName))
	if err := c.enforceNetworkCfg(ctx, remoteClusterID, false); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while creating network configuration in remote cluster: %v", err))
		return err
	}
	s.Success(fmt.Sprintf("network configuration created in remote cluster {%s}", remoteClusterID.ClusterName))

	// Wait for the network configuration to be processed by the remote cluster.
	s, _ = c.printer.Spinner.Start(fmt.Sprintf("waiting network configuration to be processed by remote cluster {%s}", remoteClusterID.ClusterName))
	netcfg, err := c.waitForNetCfg(ctx, remoteClusterID, 60*time.Second)
	if err != nil {
		s.Fail(err)
		return err
	}
	s.UpdateText(fmt.Sprintf("reflecting network configuration status from cluster {%s}", remoteClusterID.ClusterName))
	if err := c.reflectNetworkCfgStatus(ctx, remoteClusterID, &netcfg.Status); err != nil {
		s.Fail(err)
		return err
	}
	s.Success(fmt.Sprintf("network configuration status correctly reflected from cluster {%s}", remoteClusterID.ClusterName))
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
		cl = c.locCtrlRunClient
		if selector, err = labelSelectorForReplicatedResource(remoteClusterID, local); err != nil {
			return nil, err
		}
		ns = c.locTenantNamespace
	} else {
		cl = c.remCtrlRunClient
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
			Name:      foreigncluster.UniqueName(remoteClusterID),
			Namespace: c.locTenantNamespace,
			Labels: map[string]string{
				liqoconsts.ReplicationRequestedLabel:   strconv.FormatBool(true),
				liqoconsts.ReplicationDestinationLabel: remoteClusterID.ClusterID,
			},
		},
	}
	c.populateNetworkCfg(netcfg, remoteClusterID, local)

	if local {
		return c.locCtrlRunClient.Create(ctx, netcfg)
	}

	return c.remCtrlRunClient.Create(ctx, netcfg)
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
		return c.locCtrlRunClient.Update(ctx, netcfg)
	}

	return c.remCtrlRunClient.Update(ctx, netcfg)
}

// updateNetworkCfgStatus given the remote networkconfigs resource the status is reflected on the
// local instance.
func (c *Cluster) updateNetworkCfgStatus(ctx context.Context, netcfg *netv1alpha1.NetworkConfig,
	newStatus *netv1alpha1.NetworkConfigStatus) error {
	if reflect.DeepEqual(netcfg.Status, newStatus) {
		return nil
	}

	netcfg.Status = *newStatus
	return c.locCtrlRunClient.Status().Update(ctx, netcfg)
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
			return nil, fmt.Errorf("timout (%.0fs) expired while waiting for cluster {%s} to process the networkconfig",
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
	s, _ := c.printer.Spinner.Start("port-forwarding IPAM service")

	if err := c.PortForwardOpts.RunPortForward(ctx); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while port-forwarding IPAM service: %v", err))
		return err
	}
	s.Success(fmt.Sprintf("IPAM service correctly port-forwarded {%s}", c.PortForwardOpts.Ports[0]))

	return nil
}

// StopPortForwardIPAM stops the port forwarding for the IPAM service.
func (c *Cluster) StopPortForwardIPAM() {
	s, _ := c.printer.Spinner.Start("stopping IPAM service port-forward")
	c.PortForwardOpts.StopPortForward()
	s.Success(fmt.Sprintf("IPAM service port-forward correctly stopped {%s}", c.PortForwardOpts.Ports[0]))
}

// SetUpProxy configures the proxy deployment.
func (c *Cluster) SetUpProxy(ctx context.Context) error {
	s, _ := c.printer.Spinner.Start(fmt.Sprintf("configuring proxy pod {%s} and service in namespace {%s}", proxyName, c.namespace))

	ep, err := createProxyDeployment(ctx, c.locK8sClient, proxyName, c.namespace)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while setting up proxy {%s} in namespace {%s}: %v", proxyName, c.namespace, err))
		return err
	}
	s.Success(fmt.Sprintf("proxy {%s} correctly configured in namespace {%s}", proxyName, c.namespace))

	c.proxyEP = ep

	return nil
}

// MapProxyIPForCluster maps the ClusterIP address of the local proxy on the local external CIDR as seen by the remote cluster.
func (c *Cluster) MapProxyIPForCluster(ctx context.Context, ipamClient ipam.IpamClient, remoteCluster *discoveryv1alpha1.ClusterIdentity) error {
	clusterName := remoteCluster.ClusterName
	ipToBeRemapped := c.proxyEP.GetIP()

	s, _ := c.printer.Spinner.Start(fmt.Sprintf("mapping proxy ip {%s} for cluster {%s}", ipToBeRemapped, clusterName))

	ip, err := mapServiceForCluster(ctx, ipamClient, ipToBeRemapped, remoteCluster)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while mapping proxy address {%s} for cluster {%s}: %v", ipToBeRemapped, clusterName, err))
		return err
	}

	c.proxyEP.SetRemappedIP(ip)

	s.Success(fmt.Sprintf("proxy address {%s} remapped to {%s} for remote cluster {%s}", ipToBeRemapped, ip, clusterName))

	return nil
}

// UnmapProxyIPForCluster unmaps the ClusterIP address of the local proxy on the local external CIDR as seen by the remote cluster.
func (c *Cluster) UnmapProxyIPForCluster(ctx context.Context, ipamClient ipam.IpamClient, remoteCluster *discoveryv1alpha1.ClusterIdentity) error {
	clusterName := remoteCluster.ClusterName

	// TODO: this logic will be moved on the Init function once
	// the creation of the proxy deployment and service will be
	// done at install time of liqo through the helm chart.

	s, _ := c.printer.Spinner.Start(fmt.Sprintf("unmapping proxy ip for cluster {%s}", clusterName))

	selector, err := metav1.LabelSelectorAsSelector(&liqolabels.ProxyServiceLabelSelector)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving proxy endpoint: %v", err))
		return err
	}
	svc, err := liqogetters.GetServiceByLabel(ctx, c.locCtrlRunClient, c.namespace, selector)
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving proxy endpoint: %v", err))
		return err
	}
	if k8serrors.IsNotFound(err) {
		s.Warning(fmt.Sprintf("service for proxy not found, unable to unmap proxy ip for cluster {%s}", clusterName))
		return nil
	}

	ipAuth, _, err := liqogetters.RetrieveEndpointFromService(svc, corev1.ServiceTypeClusterIP, "http")
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while retrieving proxy endpoint: %v", err))
		return err
	}

	ipToBeUnmapped := ipAuth

	if err := unmapServiceForCluster(ctx, ipamClient, ipToBeUnmapped, remoteCluster); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while unmapping proxy address {%s} for cluster {%s}: %v", ipToBeUnmapped, clusterName, err))
		return err
	}

	s.Success(fmt.Sprintf("proxy address {%s} unmapped for remote cluster {%s}", ipToBeUnmapped, clusterName))

	return nil
}

// MapAuthIPForCluster maps the ClusterIP address of the local auth service on the local external CIDR as seen by the remote cluster.
func (c *Cluster) MapAuthIPForCluster(ctx context.Context, ipamClient ipam.IpamClient, remoteCluster *discoveryv1alpha1.ClusterIdentity) error {
	clusterName := remoteCluster.ClusterName
	ipToBeUnmapped := c.authEP.GetIP()

	s, _ := c.printer.Spinner.Start(fmt.Sprintf("mapping auth ip {%s} for cluster {%s}", ipToBeUnmapped, clusterName))

	ip, err := mapServiceForCluster(ctx, ipamClient, ipToBeUnmapped, remoteCluster)
	if err != nil {
		s.Fail(fmt.Sprintf("an error occurred while mapping auth address {%s} for cluster {%s}: %v", ipToBeUnmapped, clusterName, err))
		return err
	}

	c.authEP.SetRemappedIP(ip)

	s.Success(fmt.Sprintf("auth address {%s} remapped to {%s} for remote cluster {%s}", ipToBeUnmapped, ip, clusterName))

	return nil
}

// UnmapAuthIPForCluster unmaps the ClusterIP address of the local auth service on the local external CIDR as seen by the remote cluster.
func (c *Cluster) UnmapAuthIPForCluster(ctx context.Context, ipamClient ipam.IpamClient, remoteCluster *discoveryv1alpha1.ClusterIdentity) error {
	clusterName := remoteCluster.ClusterName
	ipToBeUnmapped := c.authEP.GetIP()

	s, _ := c.printer.Spinner.Start(fmt.Sprintf("unmapping auth ip {%s} for cluster {%s}", ipToBeUnmapped, clusterName))

	if err := unmapServiceForCluster(ctx, ipamClient, ipToBeUnmapped, remoteCluster); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while unmapping auth address {%s} for cluster {%s}: %v", ipToBeUnmapped, clusterName, err))
		return err
	}

	s.Success(fmt.Sprintf("auth address {%s} unmapped for remote cluster {%s}", ipToBeUnmapped, clusterName))

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
		c.printer.Error.Printf("an error occurred while creating IPAM client: %v", err)
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
	unMapRequest := &ipam.UnmapRequest{
		ClusterID: remoteCluster.ClusterID,
		Ip:        ipToBeUnmapped,
	}

	_, err := ipamClient.UnmapEndpointIP(ctx, unMapRequest)

	return err
}

// EnforceForeignCluster enforces the presence of the foreignclusters instance for a given remote cluster.
// The newly created foreigncluster has the following fields set to:
// 	* ForeignAuthURL -> the remapped ip address for the local cluster of the auth service living in the remote cluster;
//  * ForeignProxyURL -> the remapped ip address for the local cluster of the proxy service living in the remote cluster;
//  * OutgoingPeeringEnabled -> Yes
//  * NetworkingEnabled -> No, we do not want the networking to be handled by the peering process. Networking is
// 						   handled manually by the licoctl connect/disconnect commands.
func (c *Cluster) EnforceForeignCluster(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity,
	token, authURL, proxyURL string) error {
	remID := remoteClusterID.ClusterID
	remName := remoteClusterID.ClusterName
	s, _ := c.printer.Spinner.Start(fmt.Sprintf("creating foreign cluster for the remote cluster {%s}", remName))
	if c.clusterID.ClusterID == remoteClusterID.ClusterID {
		msg := fmt.Sprintf("the clusterID {%s} of remote cluster {%s} is equal to the ID of the local cluster", remID, remName)
		s.Fail(msg)
		return fmt.Errorf(msg)
	}

	if err := authenticationtoken.StoreInSecret(ctx, c.locK8sClient, remID, token, c.namespace); err != nil {
		msg := fmt.Sprintf("an error occurred while storing auth token for remote cluster {%s}: %v", remName, err)
		s.Fail(msg)
		return fmt.Errorf(msg)
	}

	// Get existing foreign cluster if it does exist
	fc, err := foreigncluster.GetForeignClusterByID(ctx, c.locCtrlRunClient, remID)
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("an error occurred while getting foreign cluster for remote cluster {%s}: %v", remName, err))
		return err
	}

	// Not nil only if the foreign cluster does not exist.
	if err != nil {
		fc = &discoveryv1alpha1.ForeignCluster{ObjectMeta: metav1.ObjectMeta{Name: remName,
			Labels: map[string]string{discovery.ClusterIDLabel: remID}}}
	}

	if _, err = controllerutil.CreateOrPatch(ctx, c.locCtrlRunClient, fc, func() error {
		fc.Spec.ForeignAuthURL = authURL
		fc.Spec.ForeignProxyURL = proxyURL
		fc.Spec.OutgoingPeeringEnabled = discoveryv1alpha1.PeeringEnabledYes
		fc.Spec.NetworkingEnabled = discoveryv1alpha1.NetworkingEnabledNo
		if fc.Spec.IncomingPeeringEnabled == "" {
			fc.Spec.IncomingPeeringEnabled = discoveryv1alpha1.PeeringEnabledAuto
		}
		if fc.Spec.InsecureSkipTLSVerify == nil {
			fc.Spec.InsecureSkipTLSVerify = pointer.BoolPtr(true)
		}
		return nil
	}); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while creating/updating foreign cluster for remote cluster {%s}: %v", remName, err))
		return err
	}
	s.Success(fmt.Sprintf("foreign cluster for remote cluster {%s} correctly configured", remName))
	return nil
}

// DeleteForeignCluster deletes the foreignclusters instance for the given remote cluster.
func (c *Cluster) DeleteForeignCluster(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	remID := remoteClusterID.ClusterID
	remName := remoteClusterID.ClusterName

	s, _ := c.printer.Spinner.Start(fmt.Sprintf("deleting foreigncluster for the remote cluster {%s}", remName))
	if c.clusterID.ClusterID == remoteClusterID.ClusterID {
		msg := fmt.Sprintf("the clusterID {%s} of remote cluster {%s} is equal to the ID of the local cluster", remID, remName)
		s.Fail(msg)
		return fmt.Errorf(msg)
	}

	// Get existing foreign cluster if it does exist
	fc, err := foreigncluster.GetForeignClusterByID(ctx, c.locCtrlRunClient, remID)
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("an error occurred while getting foreign cluster for remote cluster {%s}: %v", remName, err))
		return err
	}

	if k8serrors.IsNotFound(err) {
		s.Warning(fmt.Sprintf("it seems that the foreign cluster for remote cluster {%s} has been already removed", remName))
		return nil
	}

	if err := c.locCtrlRunClient.Delete(ctx, fc); err != nil {
		s.Fail(fmt.Sprintf("an error occurred while deleting foreigncluster for remote cluster {%s}: %v", remName, err))
		return err
	}

	s.Success(fmt.Sprintf("foreigncluster deleted for remote cluster {%s}", remName))
	return nil
}

// DisablePeering disables the peering for the remote cluster by patching the foreigncusters resource.
func (c *Cluster) DisablePeering(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	remID := remoteClusterID.ClusterID
	remName := remoteClusterID.ClusterName
	s, _ := c.printer.Spinner.Start(fmt.Sprintf("disabling peering for the remote cluster {%s}", remName))
	if c.clusterID.ClusterID == remoteClusterID.ClusterID {
		msg := fmt.Sprintf("the clusterID {%s} of remote cluster {%s} is equal to the ID of the local cluster", remID, remName)
		s.Fail(msg)
		return fmt.Errorf(msg)
	}

	// Get existing foreign cluster if it does exist
	fc, err := foreigncluster.GetForeignClusterByID(ctx, c.locCtrlRunClient, remID)
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("an error occurred while getting foreign cluster for remote cluster {%s}: %v", remName, err))
		return err
	}

	// Not nil only if the foreign cluster does not exist.
	if err != nil {
		s.Warning(fmt.Sprintf("it seems that the foreign cluster for remote cluster {%s} has been already removed", remName))
		return nil
	}

	// Set outgoing peering to no.
	if _, err := controllerutil.CreateOrPatch(ctx, c.locCtrlRunClient, fc, func() error {
		fc.Spec.OutgoingPeeringEnabled = "No"
		return nil
	}); err != nil {
		s.Fail(fmt.Sprintf("an error occurred withe disabling peering for remote cluster {%s}: %v", remName, err))
		return err
	}
	s.Success(fmt.Sprintf("peering correctly disabled for remote cluster {%s}", remName))

	return nil
}

// WaitForUnpeering waits until the status on the foreiglcusters resource states that the in/outgoing peering has been successfully
// set to None or the timeout expires.
func (c *Cluster) WaitForUnpeering(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity, timeout time.Duration) error {
	remName := remoteClusterID.ClusterName
	s, _ := c.printer.Spinner.Start(fmt.Sprintf("waiting for event {%s} from the remote cluster {%s}", UnpeeringEvent, remName))
	err := WaitForEventOnForeignCluster(ctx, remoteClusterID, UnpeeringEvent, UnpeerChecker, timeout, c.locCtrlRunClient)
	if err != nil {
		s.Fail(err.Error())
		return err
	}
	s.Success(fmt.Sprintf("event {%s} successfully occurred for remote cluster {%s}", UnpeeringEvent, remName))
	return nil
}

// WaitForAuth waits until the authentication has been established with the remote cluster or the timeout expires.
func (c *Cluster) WaitForAuth(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity, timeout time.Duration) error {
	remName := remoteClusterID.ClusterName
	s, _ := c.printer.Spinner.Start(fmt.Sprintf("waiting for event {%s} from the remote cluster {%s}", AuthEvent, remName))
	err := WaitForEventOnForeignCluster(ctx, remoteClusterID, AuthEvent, AuthChecker, timeout, c.locCtrlRunClient)
	if err != nil {
		s.Fail(err.Error())
		return err
	}
	s.Success(fmt.Sprintf("event {%s} successfully occurred for remote cluster {%s}", AuthEvent, remName))
	return nil
}
