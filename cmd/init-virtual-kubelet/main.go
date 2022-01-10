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
	"os"
	"path/filepath"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
	"github.com/liqotech/liqo/pkg/utils"
	vk "github.com/liqotech/liqo/pkg/vkMachinery"
	"github.com/liqotech/liqo/pkg/vkMachinery/csr"
)

const timeout = 30 * time.Second

func main() {
	var config *rest.Config
	var distribution string
	klog.Info("Loading client config")
	flag.StringVar(&distribution, "k8s-distribution", "kubernetes", "determine the provider to adapt csr generation")
	ctx := context.Background()

	kubeconfigPath, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		kubeconfigPath = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	klog.Infof("Loading client: %s", kubeconfigPath)
	config, err := utils.UserConfig(kubeconfigPath)
	if err != nil {
		klog.Fatalf("Unable to create client config: %s", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Unable to create client: %s", err)
	}

	name, ok := os.LookupEnv("POD_NAME")
	if !ok {
		klog.Fatal("Unable to create CSR: POD_NAME undefined")
	}

	podIP, err := liqonetutils.GetPodIP()
	if err != nil {
		klog.Fatal(err)
	}

	namespace, ok := os.LookupEnv("POD_NAMESPACE")
	if !ok {
		klog.Fatal("Unable to create CSR: POD_NAMESPACE undefined")
	}

	nodeName, ok := os.LookupEnv("NODE_NAME")
	if !ok {
		klog.Fatal("Unable to create CSR: NODE_NAME undefined")
	}

	defer func() {
		if err = csr.PersistCertificates(ctx, client, nodeName, namespace,
			vk.CsrLocation, vk.KeyLocation, vk.CertLocation); err != nil {
			klog.Error(err)
			os.Exit(1)
		}
	}()

	_, hasCertificate, err := csr.GetCSRSecret(ctx, client, nodeName, namespace)
	if !apierrors.IsNotFound(err) && !hasCertificate {
		if err != nil {
			klog.Fatal(err)
		} else {
			klog.Info("Certificate already present for this nodeName. Skipping")
		}
		return
	}

	// Generate Key and CSR files in PEM format
	if err := csr.CreateCSRResource(ctx, name, client, nodeName, namespace, distribution, podIP); err != nil {
		klog.Fatalf("Unable to create CSR: %s", err)
	}

	cancelCtx, cancel := context.WithTimeout(ctx, timeout)
	csrWatcher := csr.NewWatcher(client, 0, labels.SelectorFromSet(vk.CsrLabels))
	csrWatcher.Start(ctx)
	cert, err := csrWatcher.RetrieveCertificate(cancelCtx, name)
	cancel()

	if err != nil {
		klog.Fatalf("Unable to get certificate: %w", err)
	}

	if err := csr.StoreCertificate(ctx, client, cert, namespace, nodeName); err != nil {
		klog.Fatal("Unable to store the CRT file in secret")
	}
}
