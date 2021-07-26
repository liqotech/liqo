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

package main

import (
	"context"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/log/klogv2"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/cmd/virtual-kubelet/provider"
	"github.com/liqotech/liqo/cmd/virtual-kubelet/root"
)

var (
	defaultK8sVersion = "v1.18.2" // This should follow the version of k8s.io/kubernetes we are importing
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	log.L = klogv2.New(nil)

	opts := &root.Opts{}
	if err := root.SetDefaultOpts(opts); err != nil {
		klog.Fatal(err)
	}

	s := provider.NewStore()

	rootCmd := root.NewCommand(ctx, filepath.Base(os.Args[0]), s, opts)

	root.InstallFlags(rootCmd.Flags(), opts)
	rootCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		opts.Version = getK8sVersion(ctx, opts.HomeKubeconfig)
		if opts.Profiling {
			enableProfiling()
		}
		if err := registerKubernetes(ctx, s); err != nil {
			klog.Fatal(err)
		}
		return nil
	}

	if err := rootCmd.Execute(); err != nil {
		klog.Error(err)
	}
}

func enableProfiling() {
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		klog.Info(
			http.ListenAndServe("0.0.0.0:6060", mux))
	}()
}

func getK8sVersion(ctx context.Context, defaultConfigPath string) string {
	var config *rest.Config
	var configPath string

	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--kubeconfig" {
			configPath = os.Args[i+1]
		}
	}
	if configPath == "" {
		configPath = defaultConfigPath
	}

	// Check if the kubeConfig file exists.
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		// Get the kubeconfig from the filepath.
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			klog.Warningf("Cannot read k8s version: using default version %v; error: %v", defaultK8sVersion, err)
			return defaultK8sVersion
		}
	} else {
		// Set to in-cluster config.
		config, err = rest.InClusterConfig()
		if err != nil {
			klog.Warningf("Cannot read k8s version: using default version %v; error: %v", defaultK8sVersion, err)
			return defaultK8sVersion
		}
	}

	c, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		klog.Warningf("Cannot read k8s version: using default version %v; error: %v", defaultK8sVersion, err)
		return defaultK8sVersion
	}
	v, err := c.ServerVersion()
	if err != nil {
		klog.Warningf("Cannot read k8s version: using default version %v; error: %v", defaultK8sVersion, err)
		return defaultK8sVersion
	}

	return v.GitVersion
}
