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
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/cmd/virtual-kubelet/root"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop() // Unregister the handler, so that a second signal immediately aborts the program.
	}()

	log.L = klogv2.New(nil)

	opts := root.NewOpts()
	rootCmd := root.NewCommand(ctx, filepath.Base(os.Args[0]), opts)

	root.InstallFlags(rootCmd.Flags(), opts)
	rootCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if opts.EnableProfiling {
			enableProfiling()
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
