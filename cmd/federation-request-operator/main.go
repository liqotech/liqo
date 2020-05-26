package main

import (
	"context"
	"flag"
	federation_request_operator "github.com/netgroup-polito/dronev2/internal/federation-request-operator"
	federation_request_admission "github.com/netgroup-polito/dronev2/internal/federation-request-operator/federation-request-admission"
	"os"
	"os/signal"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"syscall"
)

var (
	mainLog = ctrl.Log.WithName("main")
)

func main() {
	mainLog.Info("Starting")

	var namespace string

	flag.StringVar(&namespace, "namespace", "default", "Namespace where your configs are stored.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT)

	mainLog.Info("Starting admission webhook")
	srv := federation_request_admission.StartWebhook()

	mainLog.Info("Starting federation-request operator")
	go federation_request_operator.StartOperator(namespace)

	<-sig

	_ = srv.Server.Shutdown(context.Background())
}
