// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package root

import (
	"flag"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

// InstallFlags configures the virtual kubelet flags.
func InstallFlags(flags *pflag.FlagSet, o *Opts) {
	flags.StringVar(&o.HomeKubeconfig, "home-kubeconfig", o.HomeKubeconfig, "kube config file to use for connecting to the Kubernetes API server")
	flags.StringVar(&o.RemoteKubeconfigSecretName, "foreign-kubeconfig-secret-name", o.RemoteKubeconfigSecretName,
		"Secret name to use for connecting to the remote Kubernetes API server")
	flags.StringVar(&o.NodeName, "nodename", o.NodeName, "The name of the node registered by the virtual kubelet")
	flags.StringVar(&o.TenantNamespace, "tenant-namespace", o.TenantNamespace, "The tenant namespace associated with the remote cluster")
	flags.DurationVar(&o.InformerResyncPeriod, "resync-period", o.InformerResyncPeriod, "The resync period for the informers")

	flags.StringVar(&o.HomeCluster.ClusterID, "home-cluster-id", o.HomeCluster.ClusterID, "The ID of the home cluster")
	flags.StringVar(&o.HomeCluster.ClusterName, "home-cluster-name", o.HomeCluster.ClusterName, "The name of the home cluster")
	flags.StringVar(&o.ForeignCluster.ClusterID, "foreign-cluster-id", o.ForeignCluster.ClusterID, "The ID of the foreign cluster")
	flags.StringVar(&o.ForeignCluster.ClusterName, "foreign-cluster-name", o.ForeignCluster.ClusterName, "The name of the foreign cluster")
	flags.StringVar(&o.LiqoIpamServer, "ipam-server", o.LiqoIpamServer,
		"The address to contact the IPAM module (leave it empty to disable the IPAM module)")
	flags.BoolVar(&o.DisableIPReflection, "disable-ip-reflection", o.DisableIPReflection,
		"Disable the IP reflection for the offloaded pods")

	flags.StringVar(&o.NodeIP, "node-ip", o.NodeIP, "The IP address of the virtual kubelet pod, and assigned to the virtual node as internal address")
	flags.Var(o.CertificateType, "certificate-type", "The type of virtual kubelet server certificate to generate, among kubelet, aws, self-signed")
	flags.Uint16Var(&o.ListenPort, "listen-port", o.ListenPort, "The port to listen to for requests from the Kubernetes API server")
	flags.BoolVar(&o.EnableProfiling, "enable-profiling", o.EnableProfiling, "Enable pprof profiling")

	flags.UintVar(&o.PodWorkers, "pod-reflection-workers", o.PodWorkers, "The number of pod reflection workers")
	flags.UintVar(&o.ServiceWorkers, "service-reflection-workers", o.ServiceWorkers, "The number of service reflection workers")
	flags.UintVar(&o.EndpointSliceWorkers, "endpointslice-reflection-workers", o.EndpointSliceWorkers,
		"The number of endpointslice reflection workers")
	flags.UintVar(&o.IngressWorkers, "ingress-reflection-workers", o.IngressWorkers, "The number of ingress reflection workers")
	flags.UintVar(&o.ConfigMapWorkers, "configmap-reflection-workers", o.ConfigMapWorkers, "The number of configmap reflection workers")
	flags.UintVar(&o.SecretWorkers, "secret-reflection-workers", o.SecretWorkers, "The number of secret reflection workers")
	flags.UintVar(&o.ServiceAccountWorkers, "service-account-reflection-workers", o.ServiceAccountWorkers,
		"The number of service account reflection workers (applies only if API server support is enabled in token API mode)")
	flags.UintVar(&o.PersistentVolumeClaimWorkers, "persistentvolumeclaim-reflection-workers", o.PersistentVolumeClaimWorkers,
		"The number of persistentvolumeclaim reflection workers")
	flags.UintVar(&o.EventWorkers, "event-reflection-workers", o.EventWorkers, "The number of event reflection workers")

	flags.DurationVar(&o.NodeLeaseDuration, "node-lease-duration", o.NodeLeaseDuration, "The duration of the node leases")
	flags.DurationVar(&o.NodePingInterval, "node-ping-interval", o.NodePingInterval,
		"The interval the reachability of the remote API server is verified to assess node readiness, 0 to disable")
	flags.DurationVar(&o.NodePingTimeout, "node-ping-timeout", o.NodePingTimeout,
		"The timeout of the remote API server reachability check")
	flags.BoolVar(&o.NodeCheckNetwork, "node-check-network", o.NodeCheckNetwork,
		"Check the network connectivity to set the node status to Ready")

	flags.Var(&o.NodeExtraAnnotations, "node-extra-annotations", "Extra annotations to add to the Virtual Node")
	flags.Var(&o.NodeExtraLabels, "node-extra-labels", "Extra labels to add to the Virtual Node")

	flags.BoolVar(&o.EnableAPIServerSupport, "enable-apiserver-support", false,
		"Enable offloaded pods to interact back with the local Kubernetes API server")
	flags.BoolVar(&o.EnableStorage, "enable-storage", false, "Enable the Liqo storage reflection")
	flags.StringVar(&o.VirtualStorageClassName, "virtual-storage-class-name", "liqo", "Name of the virtual storage class")
	flags.StringVar(&o.RemoteRealStorageClassName, "remote-real-storage-class-name", "", "Name of the real storage class to use for the actual volumes")
	flags.BoolVar(&o.EnableMetrics, "metrics-enabled", false, "Enable the metrics server")
	flags.StringVar(&o.MetricsAddress, "metrics-address", ":8080", "The address to listen to for metrics requests")
	flags.StringVar(&o.HomeAPIServerHost, "home-api-server-host", "",
		"Home cluster API server HOST, this parameter is optional and required only to override the default values")
	flags.StringVar(&o.HomeAPIServerPort, "home-api-server-port", "",
		"Home cluster API server PORT, this parameter is optional and required only to override the default values")

	flagset := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(flagset)
	flagset.VisitAll(func(f *flag.Flag) {
		f.Name = "klog." + f.Name
		flags.AddGoFlag(f)
	})

	flagset = flag.NewFlagSet("restcfg", flag.PanicOnError)
	restcfg.InitFlags(flagset)
	flags.AddGoFlagSet(flagset)
}
