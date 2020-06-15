package main

import (
	"flag"
	"github.com/joho/godotenv"
	peering_request_operator "github.com/liqoTech/liqo/internal/peering-request-operator"
	peering_request_admission "github.com/liqoTech/liqo/internal/peering-request-operator/peering-request-admission"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	mainLog = ctrl.Log.WithName("main")
)

func main() {
	mainLog.Info("Starting")

	var liqoConfigmap, broadcasterImage, broadcasterServiceAccount string
	var inputEnvFile string

	flag.StringVar(&inputEnvFile, "input-env-file", "/etc/environment/liqo/env", "The environment variable file to source at startup")
	flag.StringVar(&liqoConfigmap, "config-map", "liqo-configmap", "Liqo ConfigMap name")
	flag.StringVar(&broadcasterImage, "broadcaster-image", "liqo/advertisement-broadcaster", "Broadcaster-operator image name")
	flag.StringVar(&broadcasterServiceAccount, "broadcaster-sa", "broadcaster", "Broadcaster-operator ServiceAccount name")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	if err := godotenv.Load(inputEnvFile); err != nil {
		mainLog.Error(err, "The env variable file hasn't been correctly loaded")
		os.Exit(1)
	}

	namespace, ok := os.LookupEnv("liqonamespace")
	if !ok {
		namespace = "default"
	}
	certPath, ok := os.LookupEnv("liqocert")
	if !ok {
		certPath = "/etc/ssl/liqo/server-cert.pem"
	}
	keyPath, ok := os.LookupEnv("liqokey")
	if !ok {
		certPath = "/etc/ssl/liqo/server-key.pem"
	}

	mainLog.Info("Starting admission webhook")
	_ = peering_request_admission.StartWebhook(certPath, keyPath, namespace)

	mainLog.Info("Starting peering-request operator")
	peering_request_operator.StartOperator(namespace, liqoConfigmap, broadcasterImage, broadcasterServiceAccount)
}
