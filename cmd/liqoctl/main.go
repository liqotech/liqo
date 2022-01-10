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

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqocmd "github.com/liqotech/liqo/cmd/liqoctl/cmd"
)

const (
	terminationTimeout = 5 * time.Second
)

func init() {
	_ = discoveryv1alpha1.AddToScheme(scheme.Scheme)
	_ = offloadingv1alpha1.AddToScheme(scheme.Scheme)
}

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-ctx.Done()
		<-time.After(terminationTimeout)
		os.Exit(1)
	}()

	cmd := liqocmd.NewRootCommand(ctx)
	msg := cmd.ExecuteContext(ctx)
	if msg != nil {
		os.Exit(1)
	}
}
