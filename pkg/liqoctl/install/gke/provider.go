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
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/option"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

var _ install.Provider = (*Options)(nil)

// Options encapsulates the arguments of the install command.
type Options struct {
	*install.Options

	credentialsPath string
	projectID       string
	zone            string
	clusterID       string
}

// New initializes a new Provider object.
func New(o *install.Options) install.Provider {
	return &Options{Options: o}
}

// Name returns the name of the provider.
func (o *Options) Name() string { return "gke" }

// Examples returns the examples string for the given provider.
func (o *Options) Examples() string {
	return `Examples:
  $ {{ .Executable }} install gke --credentials-path ~/.liqo/gcp_service_account \
      --cluster-id foo --project-id bar --zone europe-west-1b
`
}

// RegisterFlags registers the flags for the given provider.
func (o *Options) RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.credentialsPath, "credentials-path", "",
		"The path to the GCP credentials JSON file (c.f. https://cloud.google.com/docs/authentication/production#create_service_account")
	cmd.Flags().StringVar(&o.projectID, "project-id", "", "The GCP project where the cluster is deployed in")
	cmd.Flags().StringVar(&o.clusterID, "cluster-id", "", "The GKE clusterID of the cluster")
	cmd.Flags().StringVar(&o.zone, "zone", "", "The GCP zone where the cluster is running")

	utilruntime.Must(cmd.MarkFlagRequired("credentials-path"))
	utilruntime.Must(cmd.MarkFlagRequired("project-id"))
	utilruntime.Must(cmd.MarkFlagRequired("cluster-id"))
	utilruntime.Must(cmd.MarkFlagRequired("zone"))
}

// Initialize performs the initialization tasks to retrieve the provider-specific parameters.
func (o *Options) Initialize(ctx context.Context) error {
	o.Printer.Verbosef("GKE Credentials Path: %q", o.credentialsPath)
	o.Printer.Verbosef("GKE ProjectID: %q", o.projectID)
	o.Printer.Verbosef("GKE Zone: %q", o.zone)
	o.Printer.Verbosef("GKE ClusterID: %q", o.clusterID)

	svc, err := container.NewService(ctx, option.WithCredentialsFile(o.credentialsPath))
	if err != nil {
		return fmt.Errorf("failed connecting to the Google container API: %w", err)
	}

	cluster, err := svc.Projects.Zones.Clusters.Get(o.projectID, o.zone, o.clusterID).Do()
	if err != nil {
		return fmt.Errorf("failed retrieving GKE cluster information: %w", err)
	}
	o.parseClusterOutput(cluster)

	netSvc, err := compute.NewService(ctx, option.WithCredentialsFile(o.credentialsPath))
	if err != nil {
		return fmt.Errorf("failed connecting to the Google compute API: %w", err)
	}

	subnet, err := netSvc.Subnetworks.Get(o.projectID, o.getRegion(), getSubnetName(cluster.NetworkConfig.Subnetwork)).Do()
	if err != nil {
		return fmt.Errorf("failed retrieving subnets information: %w", err)
	}

	o.ReservedSubnets = append(o.ReservedSubnets, subnet.IpCidrRange)
	return nil
}

// Values returns the customized provider-specifc values file parameters.
func (o *Options) Values() map[string]interface{} {
	return map[string]interface{}{}
}

func (o *Options) parseClusterOutput(cluster *container.Cluster) {
	o.APIServer = cluster.Endpoint
	o.ServiceCIDR = cluster.ServicesIpv4Cidr
	o.PodCIDR = cluster.ClusterIpv4Cidr

	// if the cluster name has not been provided, we default it to the cloud provider resource name.
	if o.ClusterName == "" {
		o.ClusterName = cluster.Name
	}

	o.ClusterLabels[consts.TopologyRegionClusterLabel] = cluster.Location
}

func (o *Options) getRegion() string {
	strs := strings.Split(o.zone, "-")
	return strings.Join(strs[:2], "-")
}

func getSubnetName(subnetID string) string {
	strs := strings.Split(subnetID, "/")
	if len(strs) == 0 {
		return ""
	}
	return strs[len(strs)-1]
}
