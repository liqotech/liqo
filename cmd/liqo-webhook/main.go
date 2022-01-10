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
	"flag"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/mutate"
)

const gracefulPeriod = 5 * time.Second

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	config := &mutate.MutationConfig{}
	setOptions(config)

	klog.Info("Starting server ...")

	ctx, cancel := context.WithCancel(context.Background())
	ctxSignal, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)

	s, err := mutate.NewMutationServer(ctx, config)
	if err != nil {
		klog.Fatal(err)
	}

	go func() {
		defer cancel()

		<-ctxSignal.Done()
		// Restore default signal handler.
		stop()

		ctxShutdown, cancelShutdown := context.WithTimeout(ctx, gracefulPeriod)
		defer cancelShutdown()

		klog.Info("Received signal, shutting down")
		s.Shutdown(ctxShutdown)
	}()

	s.Serve()
	<-ctx.Done()
	klog.Info("Liqo webhook cleanly shutdown")
}
