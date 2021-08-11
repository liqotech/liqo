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
