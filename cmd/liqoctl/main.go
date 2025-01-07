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

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqocmd "github.com/liqotech/liqo/cmd/liqoctl/cmd"
)

func init() {
	utilruntime.Must(liqov1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(offloadingv1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(networkingv1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(ipamv1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(authv1beta1.AddToScheme(scheme.Scheme))
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-ctx.Done()
		stop() // Reset the default behavior if further signals are received.
	}()

	cmd := liqocmd.NewRootCommand(ctx)
	if err := cmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
