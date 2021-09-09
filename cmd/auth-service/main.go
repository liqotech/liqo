// Copyright 2019-2021 The Liqo Authors
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
	"flag"
	"os"
	"time"

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	authservice "github.com/liqotech/liqo/internal/auth-service"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

func main() {
	klog.Info("Starting")

	var awsConfig identitymanager.AwsConfig

	namespace := flag.String("namespace", "default", "Namespace where your configs are stored.")
	resync := flag.Duration("resync-period", 30*time.Second, "The resync period for the informers")

	address := flag.String("address", ":5000", "The address the service binds to")
	certPath := flag.String("cert-path", "/certs/cert.pem", "The path to the TLS certificate")
	keyPath := flag.String("key-path", "/certs/key.pem", "The path to TLS private key")
	useTLS := flag.Bool("enable-tls", false, "Enable HTTPS server")

	clusterName := flag.String("advertise-cluster-name", "", "The cluster name advertised during the peering process")
	enableAuth := flag.Bool("enable-authentication", true,
		"Whether to authenticate remote clusters through tokens before granting an identity (warning: disable only for testing purposes)")

	flag.StringVar(&awsConfig.AwsAccessKeyID, "aws-access-key-id", "", "AWS IAM AccessKeyID for the Liqo User")
	flag.StringVar(&awsConfig.AwsSecretAccessKey, "aws-secret-access-key", "", "AWS IAM SecretAccessKey for the Liqo User")
	flag.StringVar(&awsConfig.AwsRegion, "aws-region", "", "AWS region where the local cluster is running")
	flag.StringVar(&awsConfig.AwsClusterName, "aws-cluster-name", "", "Name of the local EKS cluster")

	// Configure the flags concerning the exposed API server connection parameters.
	apiserver.InitFlags(nil)

	restcfg.InitFlags(nil)
	klog.InitFlags(nil)
	flag.Parse()

	klog.Info("Namespace: ", *namespace)

	config := restcfg.SetRateLimiter(ctrl.GetConfigOrDie())

	authService, err := authservice.NewAuthServiceCtrl(
		config, *namespace, awsConfig, *resync, apiserver.GetConfig(), *enableAuth, *useTLS, *clusterName)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}

	if err = authService.Start(*address, *useTLS, *certPath, *keyPath); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}
