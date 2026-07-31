// Copyright 2019-2026 The Liqo Authors
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

package secretcontroller

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path"
	"time"

	adminssionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/certificate"
)

const servingCertsDir = "/tmp/k8s-webhook-server/serving-certs/"

// webhookCertBundle contains the certificate material generated for the webhook server.
type webhookCertBundle struct {
	ca        []byte
	crt       []byte
	key       []byte
	notBefore time.Time
	notAfter  time.Time
}

// NewSecretReconciler returns a new SecretReconciler.
func NewSecretReconciler(
	cl client.Client,
	s *runtime.Scheme,
	liqoNamespace, secretName string,
	recorder record.EventRecorder,
) *SecretReconciler {
	return &SecretReconciler{
		Client:        cl,
		Scheme:        s,
		liqoNamespace: liqoNamespace,
		secretName:    secretName,

		eventRecorder: recorder,
	}
}

// SecretReconciler reconciles a Secret object.
type SecretReconciler struct {
	client.Client
	*runtime.Scheme

	liqoNamespace string
	secretName    string

	eventRecorder record.EventRecorder
}

// cluster-role
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;update

// Reconcile Secret resources for webhooks.
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	var secret corev1.Secret
	if err = r.Get(ctx, req.NamespacedName, &secret); err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("Secret %s not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Error(err, "unable to get Secret")
		return ctrl.Result{}, err
	}

	requeueIn, err := HandleSecret(ctx, r.Client, &secret)
	if err != nil {
		klog.Error(err, "unable to handle Secret")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: requeueIn}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	secretPredicate := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetNamespace() == r.liqoNamespace && object.GetName() == r.secretName
	})

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlSecretWebhook).
		For(&corev1.Secret{}, builder.WithPredicates(secretPredicate)).
		Complete(r)
}

// HandleSecret handles the given Secret for webhooks.
func HandleSecret(ctx context.Context, cl client.Client, secret *corev1.Secret) (time.Duration, error) {
	requeueIn := time.Duration(0)

	if secret.Annotations == nil {
		return requeueIn, fmt.Errorf("no annotations found in Secret %s/%s", secret.Namespace, secret.Name)
	}
	serviceName, serviceNameOk := secret.Annotations[consts.WebhookServiceNameAnnotationKey]
	if !serviceNameOk {
		return requeueIn, fmt.Errorf("no service name found fot Secret %s/%s. Please, set the annotation %s",
			secret.Namespace, secret.Name, consts.WebhookServiceNameAnnotationKey)
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	ca, caOk := secret.Data["ca"]
	tlsKey, tlsKeyOk := secret.Data["tls.key"]
	tlsCrt, tlsCrtOk := secret.Data["tls.crt"]

	shouldGenerate := !caOk || !tlsKeyOk || !tlsCrtOk ||
		len(ca) == 0 || len(tlsKey) == 0 || len(tlsCrt) == 0

	// Check if existing certificate requires renewal.
	if !shouldGenerate {
		shouldRenew, renewIn, err := certificate.ShouldRenewCertificate(tlsCrt)
		if err != nil {
			klog.Warningf("Unable to check webhook certificate renewal for Secret %s/%s, regenerating: %v",
				secret.Namespace, secret.Name, err)
			shouldGenerate = true
		} else {
			requeueIn = renewIn
			shouldGenerate = shouldRenew
		}
	}

	// if certificate is empty or needs renewal, generate a new one and update the Secret
	if shouldGenerate {
		klog.Infof("Generating new webhook certificate for Secret %s/%s", secret.Namespace, secret.Name)
		bundle, err := createWebhookCertBundle(serviceName, secret.Namespace)
		if err != nil {
			return requeueIn, fmt.Errorf("creating Webhook certificate bundle: %w", err)
		}
		ca, tlsCrt, tlsKey = bundle.ca, bundle.crt, bundle.key

		if err := updateSecretCerts(ctx, cl, secret, ca, tlsCrt, tlsKey); err != nil {
			return requeueIn, fmt.Errorf("enforcing generated secret: %w", err)
		}

		// Schedule the next check based on the freshly generated certificate.
		requeueIn = certificate.NextRenewalCheck(bundle.notBefore, bundle.notAfter)
		klog.Infof("Webhook certificate for Secret %s/%s generated successfully", secret.Namespace, secret.Name)
	}

	err := updateServerCerts(tlsCrt, tlsKey)
	if err != nil {
		return requeueIn, fmt.Errorf("updating server certs: %w", err)
	}

	err = patchWebhookCABundle(ctx, cl, ca)
	if err != nil {
		return requeueIn, fmt.Errorf("patching webhook CA bundle: %w", err)
	}

	return requeueIn, nil
}

func updateSecretCerts(ctx context.Context, cl client.Client, secret *corev1.Secret, ca, tlsCrt, tlsKey []byte) error {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data["ca"] = ca
	secret.Data["tls.crt"] = tlsCrt
	secret.Data["tls.key"] = tlsKey

	err := cl.Update(ctx, secret)
	if err != nil {
		return fmt.Errorf("updating Secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	return nil
}

func updateServerCerts(tlsCrt, tlsKey []byte) error {
	err := os.MkdirAll(servingCertsDir, 0o700)
	if err != nil {
		return fmt.Errorf("unable to create directory: %w", err)
	}
	err = writeFile(path.Join(servingCertsDir, "tls.crt"), bytes.NewBuffer(tlsCrt))
	if err != nil {
		return fmt.Errorf("unable to write file: %w", err)
	}
	err = writeFile(path.Join(servingCertsDir, "tls.key"), bytes.NewBuffer(tlsKey))
	if err != nil {
		return fmt.Errorf("unable to write file: %w", err)
	}

	return nil
}

func patchWebhookCABundle(ctx context.Context, cl client.Client, ca []byte) error {
	whListOptions := client.ListOptions{
		LabelSelector: client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(map[string]string{
				consts.WebHookLabel: consts.WebHookLabelValue,
			}),
		},
	}

	var mwhcList adminssionregistrationv1.MutatingWebhookConfigurationList
	if err := cl.List(ctx, &mwhcList, &whListOptions); err != nil {
		return fmt.Errorf("unable to list MutatingWebhookConfigurations: %w", err)
	}

	for i := range mwhcList.Items {
		mwhc := &mwhcList.Items[i]
		for j := range mwhc.Webhooks {
			hook := &mwhc.Webhooks[j]
			hook.ClientConfig.CABundle = ca
		}

		if err := cl.Update(ctx, mwhc); err != nil {
			return fmt.Errorf("unable to update MutatingWebhookConfiguration: %w", err)
		}
	}

	var vwhcList adminssionregistrationv1.ValidatingWebhookConfigurationList
	if err := cl.List(ctx, &vwhcList, &whListOptions); err != nil {
		return fmt.Errorf("unable to list ValidatingWebhookConfigurations: %w", err)
	}

	for i := range vwhcList.Items {
		vwhc := &vwhcList.Items[i]
		for j := range vwhc.Webhooks {
			hook := &vwhc.Webhooks[j]
			hook.ClientConfig.CABundle = ca
		}

		if err := cl.Update(ctx, vwhc); err != nil {
			return fmt.Errorf("unable to update ValidatingWebhookConfiguration: %w", err)
		}
	}

	return nil
}

// createWebhookCertBundle generates a new certificate bundle for the webhook and returns it.
func createWebhookCertBundle(serviceName, namespace string) (*webhookCertBundle, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"liqo.io"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	dnsNames := []string{
		serviceName, serviceName + "." + namespace, serviceName + "." + namespace + ".svc",
		serviceName + "." + namespace + ".svc.cluster.local"}
	commonName := serviceName + "." + namespace + ".svc.cluster.local"

	// server cert config
	notBefore := time.Now()
	notAfter := notBefore.AddDate(1, 0, 0)
	cert := &x509.Certificate{
		DNSNames:     dnsNames,
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"liqo.io"},
		},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	// server private key
	serverPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// sign the server cert
	serverCertBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &serverPrivKey.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// PEM encode the  server cert and key
	serverCertPEM := new(bytes.Buffer)
	err = pem.Encode(serverCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode server certificate: %w", err)
	}

	serverPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(serverPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(serverPrivKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode server private key: %w", err)
	}

	return &webhookCertBundle{
		ca:        certPEM,
		crt:       serverCertPEM.Bytes(),
		key:       serverPrivKeyPEM.Bytes(),
		notBefore: notBefore,
		notAfter:  notAfter,
	}, nil
}

// writeFile writes data in the file at the given path.
func writeFile(filepath string, sCert *bytes.Buffer) error {
	f, err := os.Create(filepath) //nolint:gosec // the file path is not user input
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(sCert.Bytes())
	if err != nil {
		return err
	}
	return nil
}
