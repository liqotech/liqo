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

package statuslocal

import (
	"context"
	"fmt"
	"strings"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/status"
	liqoctlutils "github.com/liqotech/liqo/pkg/liqoctl/util"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
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
	lic.addClusterIdentitySection(ctx)

	lic.addNetworkSection(ctx)

	lic.addEndpointsSection(ctx)
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

func (lic *LocalInfoChecker) addClusterIdentitySection(ctx context.Context) {
	clusterIdentitySection := lic.localInfoSection.AddSection("Cluster identity")

	clusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, lic.options.CRClient, lic.options.LiqoNamespace)
	if err != nil {
		lic.addCollectionError(fmt.Errorf("unable to get cluster identity: %w", err))
	} else {
		clusterIdentitySection.AddEntry("Cluster ID", clusterIdentity.ClusterID)
		clusterIdentitySection.AddEntry("Cluster name", clusterIdentity.ClusterName)
	}

	clusterLabelsSection := clusterIdentitySection.AddSection("Cluster labels")
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
			for k, v := range clusterLabels {
				clusterLabelsSection.AddEntry(k, v)
			}
		}
	}
}

func (lic *LocalInfoChecker) addNetworkSection(ctx context.Context) {
	networkSection := lic.localInfoSection.AddSection("Network")

	if !lic.options.InternalNetworkEnabled {
		networkSection.AddEntry("Status", string(discoveryv1alpha1.PeeringConditionStatusExternal))
	} else {
		podCIDR, err := ipamutils.GetPodCIDR(ctx, lic.options.CRClient)
		if err != nil {
			lic.addCollectionError(fmt.Errorf("unable to retrieve pod CIDR: %w", err))
		} else {
			networkSection.AddEntry("Pod CIDR", podCIDR)
		}

		serviceCIDR, err := ipamutils.GetServiceCIDR(ctx, lic.options.CRClient)
		if err != nil {
			lic.addCollectionError(fmt.Errorf("unable to retrieve service CIDR: %w", err))
		} else {
			networkSection.AddEntry("Service CIDR", serviceCIDR)
		}

		externalCIDR, err := ipamutils.GetExternalCIDR(ctx, lic.options.CRClient)
		if err != nil {
			lic.addCollectionError(fmt.Errorf("unable to retrieve external CIDR: %w", err))
		} else {
			networkSection.AddEntry("External CIDR", externalCIDR)
		}

		reservedSubnets, err := ipamutils.GetReservedSubnets(ctx, lic.options.CRClient)
		if err != nil {
			lic.addCollectionError(fmt.Errorf("unable to retrieve reserved subnets: %w", err))
		} else if len(reservedSubnets) > 0 {
			networkSection.AddEntry("Reserved Subnets", reservedSubnets...)
		}
	}
}

func (lic *LocalInfoChecker) addEndpointsSection(ctx context.Context) {
	endpointsSection := lic.localInfoSection.AddSection("Endpoints")

	if aurl, err := foreigncluster.GetHomeAuthURL(ctx, lic.options.CRClient, lic.options.LiqoNamespace); err != nil {
		lic.addCollectionError(fmt.Errorf("unable to get home auth url: %w", err))
	} else {
		endpointsSection.AddEntry("Authentication", aurl)
	}

	authargs, err := liqoctlutils.RetrieveLiqoAuthDeploymentArgs(ctx, lic.options.CRClient, lic.options.LiqoNamespace)
	if err != nil {
		lic.addCollectionError(fmt.Errorf("unable to get Liqo Auth Deployment Args: %w", err))
		return
	}
	// ExtractvalueFromArgumentList errors are not handled because GetURL is able to handle void values.
	apiServerAddressArg, _ := liqoctlutils.ExtractValuesFromArgumentList("--advertise-api-server-address", authargs)
	apiServerAddress, err := apiserver.GetURL(apiServerAddressArg, lic.options.KubeClient)
	if err != nil {
		lic.addCollectionError(fmt.Errorf("unable to get api server address: %w", err))
	} else {
		endpointsSection.AddEntry("Kubernetes API server", apiServerAddress)
	}
}
