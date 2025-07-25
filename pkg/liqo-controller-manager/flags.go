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

package liqocontrollermanager

import (
	"time"

	"github.com/spf13/pflag"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/args"
)

// InitFlags adds all liqo-controller-manager flags to the given Options struct and parses them.
func InitFlags(flagset *pflag.FlagSet, opts *Options) {
	// Set up default values for pointer fields
	opts.ClusterLabels = args.StringMap{}
	opts.IngressClasses = args.ClassNameList{}
	opts.LoadBalancerClasses = args.ClassNameList{}
	opts.DefaultNodeResources = args.ResourceMap{}
	opts.GatewayServerResources = args.StringList{}
	opts.GatewayClientResources = args.StringList{}
	opts.GlobalLabels = args.StringMap{}
	opts.GlobalAnnotations = args.StringMap{}
	opts.ClusterIDFlags = args.NewClusterIDFlags(true, nil)

	// Cluster-wide modules enable/disable flags
	flagset.BoolVar(&opts.NetworkingEnabled, "networking-enabled", true, "Enable/disable the networking module")
	flagset.BoolVar(&opts.AuthenticationEnabled, "authentication-enabled", true, "Enable/disable the authentication module")
	flagset.BoolVar(&opts.OffloadingEnabled, "offloading-enabled", true, "Enable/disable the offloading module")

	// Manager flags
	flagset.IntVar(&opts.WebhookPort, "webhook-port", 9443, "The port the webhook server binds to")
	flagset.StringVar(&opts.MetricsAddr, "metrics-address", ":8082", "The address the metric endpoint binds to")
	flagset.StringVar(&opts.ProbeAddr, "health-probe-address", ":8081", "The address the health probe endpoint binds to")
	flagset.BoolVar(&opts.LeaderElection, "enable-leader-election", false, "Enable leader election for controller manager")

	// Global parameters
	flagset.DurationVar(&opts.ResyncPeriod, "resync-period", 10*time.Hour, "The resync period for the informers")
	flagset.StringVar(&opts.LiqoNamespace, "liqo-namespace", consts.DefaultLiqoNamespace, "Name of the namespace where the liqo components are running")
	flagset.IntVar(&opts.ForeignClusterWorkers, "foreign-cluster-workers", 1, "The number of workers used to reconcile ForeignCluster resources.")
	flagset.DurationVar(&opts.ForeignClusterPingInterval, "foreign-cluster-ping-interval", 15*time.Second,
		"The frequency of the ForeignCluster API server readiness check. Set 0 to disable the check")
	flagset.DurationVar(&opts.ForeignClusterPingTimeout, "foreign-cluster-ping-timeout", 5*time.Second,
		"The timeout of the ForeignCluster API server readiness check")
	flagset.StringVar(&opts.DefaultLimitsEnforcement, "default-limits-enforcement", "none",
		"Defines how strict is the enforcement of the quota offered by the remote cluster. Possible values are: none, soft, hard")

	// Networking module
	flagset.StringVar(&opts.IPAMServer, "ipam-server", "", "The address of the IPAM server (set to empty string to disable IPAM)")
	flagset.Var(&opts.GatewayServerResources, "gateway-server-resources",
		"The list of resource types that implements the gateway server. They must be in the form <group>/<version>/<resource>")
	flagset.Var(&opts.GatewayClientResources, "gateway-client-resources",
		"The list of resource types that implements the gateway client. They must be in the form <group>/<version>/<resource>")
	flagset.StringVar(&opts.WgGatewayServerClusterRoleName, "wg-gateway-server-cluster-role-name", "liqo-gateway",
		"The name of the cluster role used by the wireguard gateway servers")
	flagset.StringVar(&opts.WgGatewayClientClusterRoleName, "wg-gateway-client-cluster-role-name", "liqo-gateway",
		"The name of the cluster role used by the wireguard gateway clients")
	flagset.BoolVar(&opts.FabricFullMasqueradeEnabled, "fabric-full-masquerade-enabled", false,
		"Enable the full masquerade on the fabric network")
	flagset.BoolVar(&opts.GwmasqbypassEnabled, "gateway-masquerade-bypass-enabled", false,
		"Enable the gateway masquerade bypass")
	flagset.IntVar(&opts.NetworkWorkers, "network-ctrl-workers", 1,
		"The number of workers used to reconcile Network resources.")
	flagset.IntVar(&opts.IPWorkers, "ip-ctrl-workers", 1,
		"The number of workers used to reconcile IP resources.")
	flagset.Uint16Var(&opts.GenevePort, "geneve-port", 6081, "The port used by the Geneve tunnel")

	// Authentication module
	flagset.StringVar(&opts.APIServerAddressOverride, "api-server-address-override", "",
		"Override the API server address where the Kuberentes APIServer is exposed")
	flagset.StringVar(&opts.CAOverride, "ca-override", "", "Override the CA certificate used by Kubernetes to sign certificates (base64 encoded)")
	flagset.BoolVar(&opts.TrustedCA, "trusted-ca", false, "Whether the Kubernetes APIServer certificate is issue by a trusted CA")
	flagset.StringVar(&opts.AWSConfig.AwsAccessKeyID, "aws-access-key-id", "", "AWS IAM AccessKeyID for the Liqo User")
	flagset.StringVar(&opts.AWSConfig.AwsSecretAccessKey, "aws-secret-access-key", "", "AWS IAM SecretAccessKey for the Liqo User")
	flagset.StringVar(&opts.AWSConfig.AwsRegion, "aws-region", "", "AWS region where the local cluster is running")
	flagset.StringVar(&opts.AWSConfig.AwsClusterName, "aws-cluster-name", "", "Name of the local EKS cluster")
	flagset.Var(&opts.ClusterLabels, consts.ClusterLabelsParameter,
		"The set of labels which characterizes the local cluster when exposed remotely as a virtual node")
	flagset.Var(&opts.IngressClasses, "ingress-classes", "List of ingress classes offered by the cluster. Example: \"nginx;default,traefik\"")
	flagset.Var(&opts.LoadBalancerClasses, "load-balancer-classes", "List of load balancer classes offered by the cluster. Example:\"metallb;default\"")
	flagset.Var(&opts.DefaultNodeResources, "default-node-resources", "Default resources assigned to the Virtual Node Pod")
	flagset.Var(&opts.GlobalLabels, "global-labels", "The set of labels that will be added to all resources created by Liqo controllers")
	flagset.Var(&opts.GlobalAnnotations, "global-annotations", "The set of annotations that will be added to all resources created by Liqo controllers")

	// Offloading module
	flagset.BoolVar(&opts.EnableStorage, "enable-storage", false, "enable the liqo virtual storage class")
	flagset.StringVar(&opts.VirtualStorageClassName, "virtual-storage-class-name", "liqo", "Name of the virtual storage class")
	flagset.StringVar(&opts.RealStorageClassName, "real-storage-class-name", "", "Name of the real storage class to use for the actual volumes")
	flagset.StringVar(&opts.StorageNamespace, "storage-namespace", "liqo-storage", "Namespace where the liqo storage-related resources are stored")
	flagset.BoolVar(&opts.EnableNodeFailureController, "enable-node-failure-controller", false, "Enable the node failure controller")
	flagset.IntVar(&opts.ShadowPodWorkers, "shadow-pod-ctrl-workers", 10, "The number of workers used to reconcile ShadowPod resources.")
	flagset.IntVar(&opts.ShadowEndpointSliceWorkers, "shadow-endpointslice-ctrl-workers", 10,
		"The number of workers used to reconcile ShadowEndpointSlice resources.")

	// Cross module
	flagset.BoolVar(&opts.EnableAPIServerIPRemapping, "enable-api-server-ip-remapping", true, "Enable the API server IP remapping")
}
