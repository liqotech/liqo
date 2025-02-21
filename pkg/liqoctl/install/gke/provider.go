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

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

var _ install.Provider = (*Options)(nil)

// Options encapsulates the arguments of the install gke command.
type Options struct {
	*install.Options

	credentialsPath string
	projectID       string
	zone            string
	region          string
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
or (regional cluster)
  $ {{ .Executable }} install gke --credentials-path ~/.liqo/gcp_service_account \
      --cluster-id foo --project-id bar --region europe-west-1
`
}

// RegisterFlags registers the flags for the given provider.
func (o *Options) RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.credentialsPath, "credentials-path", "",
		"The path to the GCP credentials JSON file (c.f. https://cloud.google.com/docs/authentication/production#create_service_account")
	cmd.Flags().StringVar(&o.projectID, "project-id", "", "The GCP project where the cluster is deployed in")
	cmd.Flags().StringVar(&o.clusterID, "cluster-id", "", "The GKE clusterID of the cluster")
	cmd.Flags().StringVar(&o.zone, "zone", "", "The GCP zone where the cluster is running")
	cmd.Flags().StringVar(&o.region, "region", "", "The GCP region where the cluster is running")

	utilruntime.Must(cmd.MarkFlagRequired("credentials-path"))
	utilruntime.Must(cmd.MarkFlagRequired("project-id"))
	utilruntime.Must(cmd.MarkFlagRequired("cluster-id"))
}

// Initialize performs the initialization tasks to retrieve the provider-specific parameters.
func (o *Options) Initialize(ctx context.Context) error {
	o.Printer.Verbosef("GKE Credentials Path: %q", o.credentialsPath)
	o.Printer.Verbosef("GKE ProjectID: %q", o.projectID)
	o.Printer.Verbosef("GKE Zone: %q", o.zone)
	o.Printer.Verbosef("GKE Region: %q", o.region)
	o.Printer.Verbosef("GKE ClusterID: %q", o.clusterID)

	location, err := o.getLocation()
	if err != nil {
		return err
	}

	svc, err := container.NewService(ctx, option.WithCredentialsFile(o.credentialsPath))
	if err != nil {
		return fmt.Errorf("failed connecting to the Google container API: %w", err)
	}

	name := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", o.projectID, location, o.clusterID)
	cluster, err := svc.Projects.Locations.Clusters.Get(name).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed retrieving GKE cluster information: %w", err)
	}
	if err = o.checkFeatures(cluster); err != nil {
		return fmt.Errorf("failed checking GKE cluster features: %w", err)
	}
	o.parseClusterOutput(cluster)

	netSvc, err := compute.NewService(ctx, option.WithCredentialsFile(o.credentialsPath))
	if err != nil {
		return fmt.Errorf("failed connecting to the Google compute API: %w", err)
	}

	subnet, err := netSvc.Subnetworks.Get(
		getSubnetProjectID(cluster.NetworkConfig.Subnetwork),
		o.getRegion(),
		getSubnetName(cluster.NetworkConfig.Subnetwork),
	).
		Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed retrieving subnets information: %w", err)
	}

	o.ReservedSubnets = append(o.ReservedSubnets, subnet.IpCidrRange)
	return nil
}

// Values returns the customized provider-specifc values file parameters.
func (o *Options) Values() map[string]interface{} {
	return map[string]interface{}{
		"ipam": map[string]interface{}{
			"pools": []interface{}{
				"10.0.0.0/8",
				"192.168.0.0/16",
				"172.16.0.0/12",
				"34.118.224.0/20",
			},
		},
	}
}

func (o *Options) checkFeatures(cluster *container.Cluster) error {
	if cluster.IpAllocationPolicy.UseRoutes {
		return fmt.Errorf("IP allocation policy is set to use routes. Liqo currently supports VPC-native traffic routing clusters only")
	}
	return nil
}

func (o *Options) getLocation() (string, error) {
	switch {
	case o.zone != "" && o.region != "":
		return "", fmt.Errorf("cannot specify both --zone and --region at the same time")
	case o.zone != "":
		return o.zone, nil
	case o.region != "":
		return o.region, nil
	default:
		return "", fmt.Errorf("either --zone or --region must be specified")
	}
}

func (o *Options) parseClusterOutput(cluster *container.Cluster) {
	o.APIServer = cluster.Endpoint
	o.ServiceCIDR = cluster.ServicesIpv4Cidr
	o.PodCIDR = cluster.ClusterIpv4Cidr

	// if the cluster name has not been provided, we default it to the cloud provider resource name.
	if o.ClusterID == "" {
		o.ClusterID = liqov1beta1.ClusterID(cluster.Name)
	}

	o.ClusterLabels[consts.TopologyRegionClusterLabel] = cluster.Location
}

func (o *Options) getRegion() string {
	if o.region != "" {
		return o.region
	}
	strs := strings.Split(o.zone, "-")
	return strings.Join(strs[:2], "-")
}

func getSubnetProjectID(subnetPath string) string {
	strs := strings.Split(subnetPath, "/")
	if len(strs) == 0 {
		return ""
	}
	return strs[1]
}

func getSubnetName(subnetPath string) string {
	strs := strings.Split(subnetPath, "/")
	if len(strs) == 0 {
		return ""
	}
	return strs[len(strs)-1]
}
