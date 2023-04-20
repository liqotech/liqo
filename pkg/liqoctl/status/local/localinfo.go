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

package statuslocal

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/status"
	liqoctlutils "github.com/liqotech/liqo/pkg/liqoctl/util"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

// LocalInfoChecker implements the Check interface.
// holds the Localinformation about the cluster.
type LocalInfoChecker struct {
	options          *status.Options
	localInfoSection output.Section
	collectionErrors []error
}

const (
	localInfoCheckerName = "Local cluster information"
)

// NewLocalInfoChecker returns a new LocalInfoChecker.
func NewLocalInfoChecker(options *status.Options) *LocalInfoChecker {
	return &LocalInfoChecker{
		localInfoSection: output.NewRootSection(),
		options:          options,
	}
}

// Silent implements the Checker interface.
func (lic *LocalInfoChecker) Silent() bool {
	return false
}

// Collect implements the collect method of the Checker interface.
// it collects the infos of the local cluster.
func (lic *LocalInfoChecker) Collect(ctx context.Context) {
	clusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, lic.options.CRClient, lic.options.LiqoNamespace)
	if err != nil {
		lic.addCollectionError(fmt.Errorf("unable to get cluster identity: %w", err))
	}
	clusterIdentitySection := lic.localInfoSection.AddSection("Cluster identity")
	clusterIdentitySection.AddEntry("Cluster ID", clusterIdentity.ClusterID)
	clusterIdentitySection.AddEntry("Cluster name", clusterIdentity.ClusterName)

	ctrlargs, err := liqoctlutils.RetrieveLiqoControllerManagerDeploymentArgs(ctx, lic.options.CRClient, lic.options.LiqoNamespace)
	if err != nil {
		lic.addCollectionError(fmt.Errorf("unable to get Liqo Controller Manager Deployment Args: %w", err))
	} else {
		clusterLabelsArg, err := liqoctlutils.ExtractValuesFromArgumentList("--cluster-labels", ctrlargs)
		if err == nil {
			clusterLabels, err := liqoctlutils.ParseArgsMultipleValues(clusterLabelsArg, ",")
			if err != nil {
				lic.addCollectionError(fmt.Errorf("unable to get cluster labels: %w", err))
			}
			clusterLabelsSection := clusterIdentitySection.AddSection("Cluster labels")
			for k, v := range clusterLabels {
				clusterLabelsSection.AddEntry(k, v)
			}
		}
	}

	networkSection := lic.localInfoSection.AddSection("Network")

	if !lic.options.InternalNetworkEnabled {
		networkSection.AddEntry("Status", string(discoveryv1alpha1.PeeringConditionStatusExternal))
	} else {
		ipamStorage, err := liqogetters.GetIPAMStorageByLabel(ctx, lic.options.CRClient, labels.NewSelector())
		if err != nil {
			lic.addCollectionError(fmt.Errorf("unable to get ipam storage: %w", err))
		} else {
			networkSection.AddEntry("Pod CIDR", ipamStorage.Spec.PodCIDR)
			networkSection.AddEntry("Service CIDR", ipamStorage.Spec.ServiceCIDR)
			networkSection.AddEntry("External CIDR", ipamStorage.Spec.ExternalCIDR)
			if len(ipamStorage.Spec.ReservedSubnets) != 0 {
				networkSection.AddEntry("Reserved Subnets", ipamStorage.Spec.ReservedSubnets...)
			}
		}
	}

	err = lic.addEndpointsSection(ctx)
	if err != nil {
		lic.addCollectionError(fmt.Errorf("unable to get endpoints: %w", err))
	}
}

// GetTitle implements the getTitle method of the Checker interface.
// it returns the title of the checker.
func (lic *LocalInfoChecker) GetTitle() string {
	return localInfoCheckerName
}

// Format implements the format method of the Checker interface.
// it outputs the information about the local cluster in a string ready to be
// printed out.
func (lic *LocalInfoChecker) Format() string {
	text := ""
	if len(lic.collectionErrors) == 0 {
		text = lic.localInfoSection.SprintForBox(lic.options.Printer)
	} else {
		for _, cerr := range lic.collectionErrors {
			text += lic.options.Printer.Error.Sprintfln(lic.options.Printer.Paragraph.Sprintf("%s", cerr))
		}
		text = strings.TrimRight(text, "\n")
	}
	return text
}

// HasSucceeded return true if no errors have been kept.
func (lic *LocalInfoChecker) HasSucceeded() bool {
	return len(lic.collectionErrors) == 0
}

// addCollectionError adds a collection error. A collection error is an error that happens while
// collecting the status of a Liqo component.
func (lic *LocalInfoChecker) addCollectionError(err error) {
	lic.collectionErrors = append(lic.collectionErrors, err)
}

func (lic *LocalInfoChecker) getVpnEndpointLocalAddress(ctx context.Context) (address string, err error) {
	gwSvcSelector, err := metav1.LabelSelectorAsSelector(&liqolabels.GatewayServiceLabelSelector)
	if err != nil {
		return "", err
	}
	gwSvc, err := liqogetters.GetServiceByLabel(ctx, lic.options.CRClient, lic.options.LiqoNamespace, gwSvcSelector)
	if err != nil {
		return "", err
	}
	vpnEndpointLocalIP, vpnEndpointLocalPort, err := liqogetters.RetrieveWGEPFromService(
		gwSvc, liqoconsts.GatewayServiceAnnotationKey, liqoconsts.DriverName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("udp://%s:%s", vpnEndpointLocalIP, vpnEndpointLocalPort), nil
}

func (lic *LocalInfoChecker) addEndpointsSection(ctx context.Context) error {
	var err error
	endpointsSection := lic.localInfoSection.AddSection("Endpoints")

	if lic.options.InternalNetworkEnabled {
		var ep string
		if ep, err = lic.getVpnEndpointLocalAddress(ctx); err != nil {
			return fmt.Errorf("unable to get vpn endpoint local address: %w", err)
		}
		endpointsSection.AddEntry("Network gateway", ep)
	}

	var aurl string
	if aurl, err = foreigncluster.GetHomeAuthURL(ctx, lic.options.CRClient, lic.options.LiqoNamespace); err != nil {
		return fmt.Errorf("unable to get home auth url: %w", err)
	}
	endpointsSection.AddEntry("Authentication", aurl)

	authargs, err := liqoctlutils.RetrieveLiqoAuthDeploymentArgs(ctx, lic.options.CRClient, lic.options.LiqoNamespace)
	if err != nil {
		return fmt.Errorf("unable to get Liqo Auth Deployment Args: %w", err)
	}
	// ExtractvalueFromArgumentList errors are not handled because GetURL is able to handle void values.
	apiServerAddressArg, _ := liqoctlutils.ExtractValuesFromArgumentList("--advertise-api-server-address", authargs)
	apiServerAddress, err := apiserver.GetURL(apiServerAddressArg, lic.options.KubeClient)
	if err != nil {
		return fmt.Errorf("unable to get api server address: %w", err)
	}
	endpointsSection.AddEntry("Kubernetes API server", apiServerAddress)
	return nil
}
