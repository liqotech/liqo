package main

import (
	"flag"
	"os"
	"path/filepath"
	"time"

	"k8s.io/klog/v2"

	authservice "github.com/liqotech/liqo/internal/auth-service"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

func main() {
	klog.Info("Starting")

	var namespace string
	var kubeconfigPath string
	var resyncSeconds int64
	var listeningPort string
	var certFile string
	var keyFile string
	var useTLS bool

	var awsConfig identitymanager.AwsConfig

	flag.StringVar(&namespace, "namespace", "default", "Namespace where your configs are stored.")
	flag.StringVar(&kubeconfigPath, "kubeconfigPath",
		filepath.Join(os.Getenv("HOME"), ".kube", "config"), "For debug purpose, set path to local kubeconfig")
	flag.Int64Var(&resyncSeconds, "resyncSeconds", 30, "Resync seconds for the informers")
	flag.StringVar(&listeningPort, "listeningPort", "5000", "Sets the port where the service will listen")
	flag.StringVar(&certFile, "certFile", "/certs/cert.pem", "Path to cert file")
	flag.StringVar(&keyFile, "keyFile", "/certs/key.pem", "Path to key file")
	flag.BoolVar(&useTLS, "useTls", false, "Enable HTTPS server")

	flag.StringVar(&awsConfig.AwsAccessKeyID, "awsAccessKeyId", "", "AWS IAM AccessKeyID for the Liqo User")
	flag.StringVar(&awsConfig.AwsSecretAccessKey, "awsSecretAccessKey", "", "AWS IAM SecretAccessKey for the Liqo User")
	flag.StringVar(&awsConfig.AwsRegion, "awsRegion", "", "AWS region where the local cluster is running")
	flag.StringVar(&awsConfig.AwsClusterName, "awsClusterName", "", "Name of the local EKS cluster")

	restcfg.InitFlags(nil)
	klog.InitFlags(nil)
	flag.Parse()

	klog.Info("Namespace: ", namespace)

	authService, err := authservice.NewAuthServiceCtrl(
		namespace, kubeconfigPath, awsConfig, time.Duration(resyncSeconds)*time.Second, useTLS)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	authService.GetAuthServiceConfig(kubeconfigPath)

	if err = authService.Start(listeningPort, certFile, keyFile); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}
