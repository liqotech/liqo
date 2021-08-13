package gke

import (
	"context"

	flag "github.com/spf13/pflag"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/option"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

const (
	providerPrefix = "gke"
)

type gkeProvider struct {
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
	return &gkeProvider{}
}

// ValidateCommandArguments validates specific arguments passed to the install command.
func (k *gkeProvider) ValidateCommandArguments(flags *flag.FlagSet) (err error) {
	k.credentialsPath, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "credentials-path")
	if err != nil {
		return err
	}
	klog.V(3).Infof("GKE Credentials Path: %v", k.credentialsPath)

	k.projectID, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "project-id")
	if err != nil {
		return err
	}
	klog.V(3).Infof("GKE ProjectID: %v", k.projectID)

	k.zone, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "zone")
	if err != nil {
		return err
	}
	klog.V(3).Infof("GKE Zone: %v", k.zone)

	k.clusterID, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "cluster-id")
	if err != nil {
		return err
	}
	klog.V(3).Infof("GKE ClusterID: %v", k.clusterID)

	return nil
}

// ExtractChartParameters fetches the parameters used to customize the Liqo installation on a specific cluster of a
// given provider.
func (k *gkeProvider) ExtractChartParameters(ctx context.Context, _ *rest.Config) error {
	svc, err := container.NewService(ctx, option.WithCredentialsFile(k.credentialsPath))
	if err != nil {
		return err
	}

	cluster, err := svc.Projects.Zones.Clusters.Get(k.projectID, k.zone, k.clusterID).Do()
	if err != nil {
		return err
	}

	k.parseClusterOutput(cluster)

	return nil
}

// UpdateChartValues patches the values map with the values required for the selected cluster.
func (k *gkeProvider) UpdateChartValues(values map[string]interface{}) {
	values["apiServer"] = map[string]interface{}{
		"address": k.endpoint,
	}
	values["networkManager"] = map[string]interface{}{
		"config": map[string]interface{}{
			"serviceCIDR": k.serviceCIDR,
			"podCIDR":     k.podCIDR,
		},
	}
}

// GenerateFlags generates the set of specific subpath and flags are accepted for a specific provider.
func GenerateFlags(flags *flag.FlagSet) {
	subFlag := flag.NewFlagSet(providerPrefix, flag.ExitOnError)
	subFlag.SetNormalizeFunc(func(f *flag.FlagSet, name string) flag.NormalizedName {
		return flag.NormalizedName(installutils.PrefixedName(providerPrefix, name))
	})

	subFlag.String("credentials-path", "", "Path to the GCP credentials JSON file, "+
		"see https://cloud.google.com/docs/authentication/production#create_service_account for further details")
	subFlag.String("project-id", "", "The GCP project where your cluster is deployed in")
	subFlag.String("zone", "", "The GCP zone where your cluster is running")
	subFlag.String("cluster-id", "", "The GKE clusterID of your cluster")

	flags.AddFlagSet(subFlag)
}

func (k *gkeProvider) parseClusterOutput(cluster *container.Cluster) {
	k.endpoint = cluster.Endpoint
	k.serviceCIDR = cluster.ServicesIpv4Cidr
	k.podCIDR = cluster.ClusterIpv4Cidr
}
