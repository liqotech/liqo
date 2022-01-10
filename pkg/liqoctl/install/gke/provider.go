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

package gke

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/option"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

const (
	providerPrefix = "gke"

	credentialsPathFlag = "credentials-path"
	projectIDFlag       = "project-id"
	zoneFlag            = "zone"
	clusterIDFlag       = "cluster-id"
)

type gkeProvider struct {
	provider.GenericProvider

	credentialsPath string

	projectID string
	zone      string
	clusterID string

	endpoint    string
	serviceCIDR string
	podCIDR     string
}

// NewProvider initializes a new GKE provider struct.
func NewProvider() provider.InstallProviderInterface {
	return &gkeProvider{
		GenericProvider: provider.GenericProvider{
			ClusterLabels: map[string]string{
				consts.ProviderClusterLabel: providerPrefix,
			},
		},
	}
}

// ValidateCommandArguments validates specific arguments passed to the install command.
func (k *gkeProvider) ValidateCommandArguments(flags *flag.FlagSet) (err error) {
	k.credentialsPath, err = flags.GetString(credentialsPathFlag)
	if err != nil {
		return err
	}
	klog.V(3).Infof("GKE Credentials Path: %v", k.credentialsPath)

	k.projectID, err = flags.GetString(projectIDFlag)
	if err != nil {
		return err
	}
	klog.V(3).Infof("GKE ProjectID: %v", k.projectID)

	k.zone, err = flags.GetString(zoneFlag)
	if err != nil {
		return err
	}
	klog.V(3).Infof("GKE Zone: %v", k.zone)

	k.clusterID, err = flags.GetString(clusterIDFlag)
	if err != nil {
		return err
	}
	klog.V(3).Infof("GKE ClusterID: %v", k.clusterID)

	return nil
}

// ExtractChartParameters fetches the parameters used to customize the Liqo installation on a specific cluster of a
// given provider.
func (k *gkeProvider) ExtractChartParameters(ctx context.Context, config *rest.Config, commonArgs *provider.CommonArguments) error {
	svc, err := container.NewService(ctx, option.WithCredentialsFile(k.credentialsPath))
	if err != nil {
		return err
	}

	cluster, err := svc.Projects.Zones.Clusters.Get(k.projectID, k.zone, k.clusterID).Do()
	if err != nil {
		return err
	}

	k.parseClusterOutput(cluster)

	if !commonArgs.DisableEndpointCheck {
		if valid, err := installutils.CheckEndpoint(k.endpoint, config); err != nil {
			return err
		} else if !valid {
			return fmt.Errorf("the retrieved cluster information and the cluster selected in the kubeconfig do not match")
		}
	}

	netSvc, err := compute.NewService(ctx, option.WithCredentialsFile(k.credentialsPath))
	if err != nil {
		return err
	}

	region := k.getRegion()
	subnet, err := netSvc.Subnetworks.Get(k.projectID, region, getSubnetName(cluster.NetworkConfig.Subnetwork)).Do()
	if err != nil {
		return err
	}

	k.ReservedSubnets = append(k.ReservedSubnets, subnet.IpCidrRange)

	return nil
}

func getSubnetName(subnetID string) string {
	strs := strings.Split(subnetID, "/")
	if len(strs) == 0 {
		return ""
	}
	return strs[len(strs)-1]
}

func (k *gkeProvider) getRegion() string {
	strs := strings.Split(k.zone, "-")
	return strings.Join(strs[:2], "-")
}

// UpdateChartValues patches the values map with the values required for the selected cluster.
func (k *gkeProvider) UpdateChartValues(values map[string]interface{}) {
	values["apiServer"] = map[string]interface{}{
		"address": k.endpoint,
	}
	values["networkManager"] = map[string]interface{}{
		"config": map[string]interface{}{
			"serviceCIDR":     k.serviceCIDR,
			"podCIDR":         k.podCIDR,
			"reservedSubnets": installutils.GetInterfaceSlice(k.ReservedSubnets),
		},
	}
	values["discovery"] = map[string]interface{}{
		"config": map[string]interface{}{
			"clusterLabels": installutils.GetInterfaceMap(k.ClusterLabels),
			"clusterName":   k.ClusterName,
		},
	}
}

// GenerateFlags generates the set of specific subpath and flags are accepted for a specific provider.
func GenerateFlags(command *cobra.Command) {
	flags := command.Flags()

	flags.String(credentialsPathFlag, "", "Path to the GCP credentials JSON file, "+
		"see https://cloud.google.com/docs/authentication/production#create_service_account for further details")
	flags.String(projectIDFlag, "", "The GCP project where your cluster is deployed in")
	flags.String(zoneFlag, "", "The GCP zone where your cluster is running")
	flags.String(clusterIDFlag, "", "The GKE clusterID of your cluster")

	utilruntime.Must(command.MarkFlagRequired(credentialsPathFlag))
	utilruntime.Must(command.MarkFlagRequired(projectIDFlag))
	utilruntime.Must(command.MarkFlagRequired(zoneFlag))
	utilruntime.Must(command.MarkFlagRequired(clusterIDFlag))
}

func (k *gkeProvider) parseClusterOutput(cluster *container.Cluster) {
	k.endpoint = cluster.Endpoint
	k.serviceCIDR = cluster.ServicesIpv4Cidr
	k.podCIDR = cluster.ClusterIpv4Cidr

	// if the cluster name has not been provided (and set in the pre-checks)
	// and we have not to generate it,
	// we default it to the cloud provider resource name.
	if k.ClusterName == "" && !k.GenerateClusterName {
		k.ClusterName = cluster.Name
	}

	k.ClusterLabels[consts.TopologyRegionClusterLabel] = cluster.Location
}
