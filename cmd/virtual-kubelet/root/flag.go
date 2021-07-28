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
	"k8s.io/klog"
)

func InstallFlags(flags *pflag.FlagSet, c *Opts) {
	flags.StringVar(&c.HomeKubeconfig, "home-kubeconfig", c.HomeKubeconfig, "kube config file to use for connecting to the Kubernetes API server")
	flags.StringVar(&c.KubeClusterDomain, "cluster-domain", c.KubeClusterDomain, "kubernetes cluster-domain (default is 'cluster.local')")
	flags.StringVar(&c.NodeName, "nodename", c.NodeName, "kubernetes node name")
	flags.StringVar(&c.Provider, "provider", c.Provider, "cloud provider")
	flags.StringVar(&c.MetricsAddr, "metrics-addr", c.MetricsAddr, "address to listen for metrics/stats requests")

	flags.IntVar(&c.PodSyncWorkers, "pod-sync-workers", c.PodSyncWorkers, `set the number of pod synchronization workers`)
	flags.BoolVar(&c.EnableNodeLease, "enable-node-lease", c.EnableNodeLease, `use node leases (1.13) for node heartbeats`)

	flags.DurationVar(&c.InformerResyncPeriod, "full-resync-period", c.InformerResyncPeriod,
		"how often to perform a full resync of pods between kubernetes and the provider")
	flags.DurationVar(&c.LiqoInformerResyncPeriod, "liqo-resync-period", c.LiqoInformerResyncPeriod,
		"how often to perform a full resync of Liqo resources informers")
	flags.DurationVar(&c.StartupTimeout, "startup-timeout", c.StartupTimeout, "How long to wait for the virtual-kubelet to start")

	flags.StringVar(&c.ForeignClusterID, "foreign-cluster-id", c.ForeignClusterID, "The Id of the foreign cluster")
	flags.StringVar(&c.KubeletNamespace, "kubelet-namespace", c.KubeletNamespace, "The namespace of the virtual kubelet")
	flags.StringVar(&c.HomeClusterID, "home-cluster-id", c.HomeClusterID, "The Id of the home cluster")
	flags.StringVar(&c.LiqoIpamServer, "ipam-server", c.LiqoIpamServer, "The server the Virtual Kubelet should"+
		"connect to in order to contact the IPAM module")
	flags.BoolVar(&c.Profiling, "enable-profiling", c.Profiling, "Enable pprof profiling")

	flagset := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(flagset)
	flagset.VisitAll(func(f *flag.Flag) {
		f.Name = "klog." + f.Name
		flags.AddGoFlag(f)
	})
}
