package main

import (
	"flag"
	peering_request_operator "github.com/liqoTech/liqo/internal/peering-request-operator"
	peering_request_admission "github.com/liqoTech/liqo/internal/peering-request-operator/peering-request-admission"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	mainLog = ctrl.Log.WithName("main")
)

func main() {
	mainLog.Info("Starting")

	var namespace, liqoConfigmap, broadcasterImage, broadcasterServiceAccount string
	var certPath string

	flag.StringVar(&namespace, "namespace", "default", "Namespace where your configs are stored.")
	flag.StringVar(&certPath, "cert-path", "", "Certificate files for webhook, needed only if run outside of cluster")
	flag.StringVar(&liqoConfigmap, "config-map", "liqo-configmap", "Liqo ConfigMap name")
	flag.StringVar(&broadcasterImage, "broadcaster-image", "liqo/advertisement-broadcaster", "Broadcaster-operator image name")
	flag.StringVar(&broadcasterServiceAccount, "broadcaster-sa", "broadcaster", "Broadcaster-operator ServiceAccount name")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mainLog.Info("Starting admission webhook")
	_ = peering_request_admission.StartWebhook(certPath, namespace)

	mainLog.Info("Starting peering-request operator")
	peering_request_operator.StartOperator(namespace, liqoConfigmap, broadcasterImage, broadcasterServiceAccount)
}
