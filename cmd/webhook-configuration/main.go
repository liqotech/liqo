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
	"fmt"
	"os"
	"path"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/liqotech/liqo/pkg/webhookConfiguration"
)

const (
	LiqoMutatingWebhookServiceName = "mutatingwebhook.liqo.io/service-name"
	defaultCertDir                 = "/etc/ssl/liqo/"
)

var (
	webhookName string
	certDir     string
)

func main() {
	var k8sClient kubernetes.Interface
	var podName, podUID string

	flag.StringVar(&webhookName, "webhook-name", "", "the name of the webhook to create the secrets for")
	flag.StringVar(&certDir, "cert-dir", defaultCertDir, "the directory in which to write the secrets for webhook backends")
	flag.Parse()
	if webhookName == "" {
		klog.Fatal("webhook-name flag is mandatory")
	}

	c, err := config.GetConfig()
	if err != nil {
		klog.Fatal(err)
	}

	k8sClient, err = kubernetes.NewForConfig(c)
	if err != nil {
		klog.Fatal(err)
	}

	if podName = os.Getenv("POD_NAME"); podName == "" {
		klog.Fatal("Unknown pod name")
	}
	if podUID = os.Getenv("POD_UID"); podUID == "" {
		klog.Fatal("Unknown pod UID")
	}

	// get the mutatingWebhookConfiguration
	mutatingWebhook, err := k8sClient.AdmissionregistrationV1().
		MutatingWebhookConfigurations().
		Get(context.TODO(), webhookName, metav1.GetOptions{})
	if err != nil {
		klog.Fatal(err)
	}
	klog.Infof("mutating webhook %s found", webhookName)

	// iterate over all the webhooks in the mutatingWebhookConfiguration
	for i, wh := range mutatingWebhook.Webhooks {
		// generate tls secrets and CA
		secrets, err := webhookConfiguration.NewSecrets(wh.Name)
		if err != nil {
			klog.Fatal(err)
		}
		klog.Infof("secrets for %s generated", wh.Name)

		// write tls secrets in a kubernetes secret
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: fmt.Sprintf("%s-", wh.ClientConfig.Service.Name),
				Namespace:    wh.ClientConfig.Service.Namespace,
				Labels: map[string]string{
					LiqoMutatingWebhookServiceName: wh.ClientConfig.Service.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Name:       podName,
						UID:        types.UID(podUID),
					},
				},
			},
			Type: "kubernetes.io/tls",
			Data: map[string][]byte{
				"tls.crt": secrets.ServerCertPEM(),
				"tls.key": secrets.ServerKeyPEM(),
			},
		}

		// dump the secrets on disk for allowing the container to read them
		if err = secrets.WriteFiles(path.Join(certDir, "tls.crt"), path.Join(certDir, "tls.key")); err != nil {
			klog.Fatal(err)
		}
		klog.Infof("secrets for %s written on disk", wh.Name)

		// create k8s secret
		if _, err := k8sClient.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
			klog.Fatal(err)
		}
		klog.Infof("secret for %s created", wh.Name)

		// patch the mutatingWebhook with the newly generated CaBundle
		caBundlePatch := []byte(fmt.Sprintf(
			`[{"op":"replace","path":"/webhooks/%d/clientConfig/caBundle","value":"%s"}]`,
			i, secrets.CAPEM()))

		klog.Info()
		_, err = k8sClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Patch(context.TODO(),
			mutatingWebhook.Name,
			types.JSONPatchType,
			caBundlePatch,
			metav1.PatchOptions{})
		if err != nil {
			klog.Fatal(err)
		}
		klog.Infof("webhook %s CaBundle patched", wh.Name)
	}
}
