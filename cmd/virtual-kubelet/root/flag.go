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
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/utils/restcfg"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/resources"
)

// InstallFlags configures the virtual kubelet flags.
func InstallFlags(flags *pflag.FlagSet, o *Opts) {
	flags.StringVar(&o.HomeKubeconfig, "home-kubeconfig", o.HomeKubeconfig, "kube config file to use for connecting to the Kubernetes API server")
	flags.StringVar(&o.RemoteKubeconfigSecretName, "foreign-kubeconfig-secret-name", o.RemoteKubeconfigSecretName,
		"Secret name to use for connecting to the remote Kubernetes API server")
	flags.StringVar(&o.NodeName, "nodename", o.NodeName, "The name of the node registered by the virtual kubelet")
	flags.StringVar(&o.TenantNamespace, "tenant-namespace", o.TenantNamespace, "The tenant namespace associated with the remote cluster")
	flags.StringVar(&o.LiqoNamespace, "liqo-namespace", o.LiqoNamespace, "The namespace where Liqo is installed")
	flags.DurationVar(&o.InformerResyncPeriod, "resync-period", o.InformerResyncPeriod, "The resync period for the informers")

	flags.Var(&o.HomeCluster, "home-cluster-id", "The ID of the home cluster")
	flags.Var(&o.ForeignCluster, "foreign-cluster-id", "The ID of the foreign cluster")
	flags.BoolVar(&o.DisableIPReflection, "disable-ip-reflection", o.DisableIPReflection,
		"Disable the IP reflection for the offloaded pods")
	flags.StringVar(&o.LocalPodCIDR, "local-podcidr", o.LocalPodCIDR, "The CIDR used for the local pods")

	flags.StringVar(&o.NodeIP, "node-ip", o.NodeIP, "The IP address of the virtual kubelet pod, and assigned to the virtual node as internal address")
	flags.Var(o.CertificateType, "certificate-type", "The type of virtual kubelet server certificate to generate, among kubelet, aws, self-signed")
	flags.Uint16Var(&o.ListenPort, "listen-port", o.ListenPort, "The port to listen to for requests from the Kubernetes API server")
	flags.BoolVar(&o.EnableProfiling, "enable-profiling", o.EnableProfiling, "Enable pprof profiling")

	setReflectorsWorkers(flags, o)
	setReflectorsType(flags, o)

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
	flags.BoolVar(&o.EnableIngress, "enable-ingress", false, "Enable the Liqo ingress reflection")
	flags.StringVar(&o.RemoteRealIngressClassName, "remote-real-ingress-class-name", "", "Name of the real ingress class to use for the actual ingress")
	flags.BoolVar(&o.EnableLoadBalancer, "enable-load-balancer", false, "Enable the Liqo load balancer reflection")
	flags.StringVar(&o.RemoteRealLoadBalancerClassName, "remote-real-load-balancer-class-name", "",
		"Name of the real load balancer class to use for the actual load balancer")
	flags.BoolVar(&o.EnableMetrics, "metrics-enabled", false, "Enable the metrics server")
	flags.StringVar(&o.MetricsAddress, "metrics-address", ":8082", "The address to listen to for metrics requests")
	flags.StringVar(&o.HomeAPIServerHost, "home-api-server-host", "",
		"Home cluster API server HOST, this parameter is optional and required only to override the default values")
	flags.StringVar(&o.HomeAPIServerPort, "home-api-server-port", "",
		"Home cluster API server PORT, this parameter is optional and required only to override the default values")
	flags.BoolVar(&o.CreateNode, "create-node", true, "Create the virtual node in the home cluster")

	flags.BoolVar(&o.VirtualKubeletLeaseEnabled, "vk-lease-enabled", true, "Enable the virtual kubelet lease")
	flags.DurationVar(&o.VirtualKubeletLeaseLeaseDuration, "vk-lease-duration", 15*time.Second,
		" The duration that non-leader candidates will wait to force acquire leadership.")
	flags.DurationVar(&o.VirtualKubeletLeaseRenewDeadline, "vk-lease-renew-interval", 10*time.Second,
		"The duration that the acting master will retry refreshing leadership before giving up.")
	flags.DurationVar(&o.VirtualKubeletLeaseRetryPeriod, "vk-lease-retry-period", 5*time.Second,
		"the duration the LeaderElector clients should wait between tries of actions.")

	restcfg.InitFlags(flags)

	flagset := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(flagset)
	flagset.VisitAll(func(f *flag.Flag) {
		f.Name = "klog." + f.Name
		flags.AddGoFlag(f)
	})

	flagset = flag.NewFlagSet("restcfg", flag.PanicOnError)
	flags.AddGoFlagSet(flagset)
}

// setReflectorsWorkers sets the flags for the number of workers used by the reflectors.
func setReflectorsWorkers(flags *pflag.FlagSet, o *Opts) {
	for i := range resources.Reflectors {
		resource := &resources.Reflectors[i]
		stringFlag := fmt.Sprintf("%s-reflection-workers", *resource)
		defaultValue := *o.ReflectorsWorkers[string(*resource)]
		usage := fmt.Sprintf("The number of workers used for the %s reflector", *resource)
		flags.UintVar(o.ReflectorsWorkers[string(*resource)], stringFlag, defaultValue, usage)
	}
}

// setReflectorsType sets the flags for the type of reflection used by the reflectors.
func setReflectorsType(flags *pflag.FlagSet, o *Opts) {
	for i := range resources.ReflectorsCustomizableType {
		resource := &resources.ReflectorsCustomizableType[i]
		stringFlag := fmt.Sprintf("%s-reflection-type", *resource)
		defaultValue := *o.ReflectorsType[string(*resource)]
		usage := fmt.Sprintf("The type of reflection used for the %s reflector", *resource)
		flags.StringVar(o.ReflectorsType[string(*resource)], stringFlag, defaultValue, usage)
	}
}
