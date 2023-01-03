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

package statuspeer

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/status"
	"github.com/liqotech/liqo/pkg/liqoctl/status/utils/resources"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
)

// PeerInfoChecker implements the Check interface.
// holds the information about the peered cluster.
type PeerInfoChecker struct {
	options            *status.Options
	peerInfoSection    output.Section
	collectionErrors   []error
	notFound           bool
	remoteClusterNames []string
}

// NewPeerInfoChecker return a new PeerInfoChecker.
func NewPeerInfoChecker(o *status.Options, remoteClusterNames ...string) *PeerInfoChecker {
	return &PeerInfoChecker{
		peerInfoSection:    output.NewRootSection(),
		options:            o,
		remoteClusterNames: remoteClusterNames,
		notFound:           false,
	}
}

// Silent implements the Checker interface.
func (pic *PeerInfoChecker) Silent() bool {
	return false
}

// Collect implements the collect method of the Checker interface.
// it collects the infos of the peered cluster.
func (pic *PeerInfoChecker) Collect(ctx context.Context) {
	localClusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, pic.options.CRClient, pic.options.LiqoNamespace)
	localClusterName := localClusterIdentity.ClusterName
	if err != nil {
		pic.addCollectionError(fmt.Errorf("unable to get local cluster identity: %w", err))
		return
	}

	foreignClusterMap, err := liqogetters.MapForeignClustersByLabel(ctx, pic.options.CRClient, labels.Everything())
	if err != nil {
		pic.addCollectionError(fmt.Errorf("unable to get foreign clusters: %w", err))
		return
	}

	foreignClusterListSelected := pic.getForeignClusterListSelected(foreignClusterMap)

	if len(foreignClusterListSelected.Items) == 0 {
		pic.peerInfoSection.AddEntry(fmt.Sprintf("Local cluster %q is not peered with any remote cluster", localClusterName))
		return
	}

	vpnLocalEndpointAddress, err := pic.getVpnEndpointLocalAddress(ctx)
	if err != nil {
		pic.addCollectionError(fmt.Errorf("unable to get local VPN endpoint address: %w", err))
		return
	}

	for i := range foreignClusterListSelected.Items {
		fc := &foreignClusterListSelected.Items[i]
		remoteClusterName := fc.Spec.ClusterIdentity.ClusterName
		// Void ClusterID is used to recognize a foreigncluster representing a remote cluster not found.
		// These foreignclusters are created in getForeignClusterListSelected function
		if fc.Spec.ClusterIdentity.ClusterID == "" {
			pic.notFound = true
			pic.peerInfoSection.AddSectionWithDetail(remoteClusterName, PeerNotFoundMsg)
			continue
		}

		remoteClusterID := fc.Spec.ClusterIdentity.ClusterID

		clusterSection := pic.peerInfoSection.AddSectionWithDetail(remoteClusterName, remoteClusterID)

		pic.addPeerSection(clusterSection, fc)

		pic.addAuthSection(clusterSection, fc)

		err = pic.addNetworkSection(ctx, clusterSection, fc, localClusterName, vpnLocalEndpointAddress)
		if err != nil {
			pic.addCollectionError(fmt.Errorf("unable to get network info for cluster %q: %w", remoteClusterName, err))
		}

		err = pic.addResourceSection(ctx, clusterSection, fc, remoteClusterID, localClusterName, remoteClusterName)
		if err != nil {
			pic.addCollectionError(fmt.Errorf("unable to get resource info for cluster %q: %w", remoteClusterName, err))
		}
	}
}

// getForeignClusterListSelected returns the list of ForeignCluster selected.
func (pic *PeerInfoChecker) getForeignClusterListSelected(
	foreignClusterMap map[string]discoveryv1alpha1.ForeignCluster) *discoveryv1alpha1.ForeignClusterList {
	foreignClusterListSelected := discoveryv1alpha1.ForeignClusterList{}
	if len(pic.remoteClusterNames) == 0 {
		foreignClusterListSelected.Items = maps.Values(foreignClusterMap)
		return &foreignClusterListSelected
	}
	for _, rcn := range pic.remoteClusterNames {
		if fc, ok := foreignClusterMap[rcn]; ok {
			foreignClusterListSelected.Items = append(foreignClusterListSelected.Items, fc)
		} else {
			foreignClusterListSelected.Items = append(foreignClusterListSelected.Items, discoveryv1alpha1.ForeignCluster{
				Spec: discoveryv1alpha1.ForeignClusterSpec{
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
						ClusterName: rcn,
					},
				},
			})
		}
	}
	return &foreignClusterListSelected
}

// addPeerSection adds a section about the peering generic info.
func (pic *PeerInfoChecker) addPeerSection(rootSection output.Section, foreignCluster *discoveryv1alpha1.ForeignCluster) {
	rootSection.AddEntry("Type", string(foreignCluster.Spec.PeeringType))
	directionSection := rootSection.AddSection("Direction")
	outgoingStatus := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.OutgoingPeeringCondition)
	directionSection.AddEntry("Outgoing", string(outgoingStatus))
	incomingStatus := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.IncomingPeeringCondition)
	directionSection.AddEntry("Incoming", string(incomingStatus))
}

// addAuthSection adds a section about the authentication status.
func (pic *PeerInfoChecker) addAuthSection(rootSection output.Section, foreignCluster *discoveryv1alpha1.ForeignCluster) {
	authSection := rootSection.AddSection("Authentication")
	authStatus := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.AuthenticationStatusCondition)
	authSection.AddEntry("Status", string(authStatus))
	if pic.options.Verbose {
		authSection.AddEntry("Auth URL", foreignCluster.Spec.ForeignAuthURL)
		if foreignCluster.Spec.ForeignProxyURL != "" {
			authSection.AddEntry("Auth Proxy URL", foreignCluster.Spec.ForeignProxyURL)
		}
	}
}

// addNetworkSection adds a section about the network configuration.
func (pic *PeerInfoChecker) addNetworkSection(ctx context.Context, rootSection output.Section,
	foreignCluster *discoveryv1alpha1.ForeignCluster, localClusterName, vpnLocalEndpointAddress string) error {
	networkSection := rootSection.AddSection("Network")
	networkStatus := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.NetworkStatusCondition)
	networkSection.AddEntry("Status", string(networkStatus))
	var localFound, remoteFound bool
	if pic.options.Verbose {
		networkConfigs, err := liqogetters.ListNetworkConfigsByLabel(ctx, pic.options.CRClient,
			foreignCluster.Status.TenantNamespace.Local, labels.NewSelector())
		if err != nil {
			return err
		}

		var selectedSection output.Section
		var remoteSectionMsg, remotePodCIDRMsg, remoteExternalCIDRMsg string
		for i := range networkConfigs.Items {
			nc := &networkConfigs.Items[i]
			if liqonetutils.IsLocalNetworkConfig(nc) {
				localFound = true
				selectedSection = networkSection.AddSection("Local CIDRs")
				remoteSectionMsg = fmt.Sprintf("how %q has been remapped by %q", localClusterName, foreignCluster.Name)
			} else {
				remoteFound = true
				selectedSection = networkSection.AddSection("Remote CIDRs")
				remoteSectionMsg = fmt.Sprintf("how %q remapped %q", localClusterName, foreignCluster.Name)
			}

			if nc.Status.PodCIDRNAT == liqoconsts.DefaultCIDRValue {
				remotePodCIDRMsg = NotRemappedMsg
			} else {
				remotePodCIDRMsg = nc.Status.PodCIDRNAT
			}
			if nc.Status.ExternalCIDRNAT == liqoconsts.DefaultCIDRValue {
				remoteExternalCIDRMsg = NotRemappedMsg
			} else {
				remoteExternalCIDRMsg = nc.Status.ExternalCIDRNAT
			}

			// Collect Original Network Configs
			originalSection := selectedSection.AddSection("Original")
			originalSection.AddEntry("Pod CIDR", nc.Spec.PodCIDR)
			originalSection.AddEntry("External CIDR", nc.Spec.ExternalCIDR)

			// Collect Remapped Network Configs
			remoteSection := selectedSection.AddSectionWithDetail("Remapped", remoteSectionMsg)
			remoteSection.AddEntry("Pod CIDR", remotePodCIDRMsg)
			remoteSection.AddEntry("External CIDR", remoteExternalCIDRMsg)
		}

		if !localFound {
			networkSection.AddSectionWithDetail("Local CIDRs", NetworkConfigNotFoundMsg)
		}
		if !remoteFound {
			networkSection.AddSectionWithDetail("Remote CIDRs", NetworkConfigNotFoundMsg)
		}
	}
	return pic.addVpnSection(ctx, networkSection, foreignCluster.Spec.ClusterIdentity,
		foreignCluster.Status.TenantNamespace.Local, vpnLocalEndpointAddress)
}

func (pic *PeerInfoChecker) getVpnEndpointLocalAddress(ctx context.Context) (address string, err error) {
	gwSvcSelector, err := metav1.LabelSelectorAsSelector(&liqolabels.GatewayServiceLabelSelector)
	if err != nil {
		return "", err
	}
	gwSvc, err := liqogetters.GetServiceByLabel(ctx, pic.options.CRClient, pic.options.LiqoNamespace, gwSvcSelector)
	if err != nil {
		return "", err
	}
	vpnEndpointLocalIP, vpnEndpointLocalPort, err := liqogetters.RetrieveWGEPFromService(
		gwSvc, liqoconsts.GatewayServiceAnnotationKey, liqoconsts.DriverName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s", vpnEndpointLocalIP, vpnEndpointLocalPort), nil
}

// addVpnSection adds a section about the VPN configuration.
func (pic *PeerInfoChecker) addVpnSection(ctx context.Context, rootSection output.Section,
	remoteClusterIdentity discoveryv1alpha1.ClusterIdentity, tenantNamespace, vpnEndpointLocalEnpointAddress string) error {
	te, err := liqogetters.GetTunnelEndpoint(ctx, pic.options.CRClient, &remoteClusterIdentity, tenantNamespace)
	if err != nil {
		if kerrors.IsNotFound(err) {
			rootSection.AddSectionWithDetail("VPN Connection", TunnelEndpointNotFoundMsg)
			return nil
		}
		return err
	}
	tunnelEndpointSection := rootSection.AddSection("VPN Connection")
	vpnEndpointSection := tunnelEndpointSection.AddSection("Gateway IPs")
	vpnEndpointSection.AddEntry("Local", vpnEndpointLocalEnpointAddress)
	vpnEndpointSection.AddEntry("Remote", fmt.Sprintf("%s:%s",
		te.Status.Connection.PeerConfiguration[liqoconsts.WgEndpointIP],
		te.Status.Connection.PeerConfiguration[liqoconsts.ListeningPort],
	))
	tunnelEndpointSection.AddEntry("Status", fmt.Sprintf("%s - %s",
		string(te.Status.Connection.Status),
		te.Status.Connection.StatusMessage),
	)
	tunnelEndpointSection.AddEntry("Latency", te.Status.Connection.Latency.Value)
	return nil
}

// addResourceSection adds a section about the resource usage.
func (pic *PeerInfoChecker) addResourceSection(ctx context.Context, rootSection output.Section,
	fc *discoveryv1alpha1.ForeignCluster, remoteClusterID, localClusterName, remoteClusterName string) error {
	resourceSection := rootSection.AddSection("Resources")

	if foreigncluster.IsOutgoingEnabled(fc) {
		resInTot, err := resources.GetAcquiredTotal(ctx, pic.options.CRClient, remoteClusterID)
		if err != nil {
			return fmt.Errorf("unable to get incoming total resources: %w", err)
		}
		inSection := resourceSection.AddSectionWithDetail(
			"Total acquired", fmt.Sprintf("resources offered by %q to %q", remoteClusterName, localClusterName))
		addResourceEntries(inSection, &resInTot)
	}

	if foreigncluster.IsIncomingEnabled(fc) {
		resOutTot, err := resources.GetSharedTotal(ctx, pic.options.CRClient, remoteClusterID)
		if err != nil {
			return fmt.Errorf("unable to get outgoing total resources: %w", err)
		}
		outSection := resourceSection.AddSectionWithDetail(
			"Total shared", fmt.Sprintf("resources offered by %q to %q", localClusterName, remoteClusterName))
		addResourceEntries(outSection, &resOutTot)
	}

	return nil
}

func addResourceEntries(section output.Section, resource *corev1.ResourceList) {
	section.AddEntry(corev1.ResourceCPU.String(), resources.CPU(*resource))
	section.AddEntry(corev1.ResourceMemory.String(), resources.Memory(*resource))
	section.AddEntry(corev1.ResourcePods.String(), resources.Pods(*resource))
	section.AddEntry(corev1.ResourceEphemeralStorage.String(), resources.EphemeralStorage(*resource))
	for k, v := range resources.Others(*resource) {
		section.AddEntry(k, v)
	}
}

// GetTitle implements the getTitle method of the Checker interface.
// it returns the title of the checker.
func (pic *PeerInfoChecker) GetTitle() string {
	return peerInfoCheckerName
}

// Format implements the format method of the Checker interface.
// it outputs the information about the peered clusters in a string ready to be
// printed out.
func (pic *PeerInfoChecker) Format() string {
	text := ""
	if len(pic.collectionErrors) == 0 {
		text = pic.peerInfoSection.SprintForBox(pic.options.Printer)
	} else {
		for _, cerr := range pic.collectionErrors {
			text += pic.options.Printer.Error.Sprintfln(pic.options.Printer.Paragraph.Sprintf("%s", cerr))
		}
		text = strings.TrimRight(text, "\n")
	}
	return text
}

// HasSucceeded return true if no errors have been kept.
func (pic *PeerInfoChecker) HasSucceeded() bool {
	return len(pic.collectionErrors) == 0 && !pic.notFound
}

// addCollectionError adds a collection error. A collection error is an error that happens while
// collecting the status of a Liqo component.
func (pic *PeerInfoChecker) addCollectionError(err error) {
	pic.collectionErrors = append(pic.collectionErrors, err)
}
