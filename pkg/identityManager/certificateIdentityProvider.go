package identitymanager

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"

	certv1 "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	certificateSigningRequest "github.com/liqotech/liqo/pkg/vkMachinery/csr"
)

// random package initialization.
func init() {
	rand.Seed(time.Now().UnixNano())
}

type certificateIdentityProvider struct {
	namespaceManager tenantnamespace.Manager
	client           kubernetes.Interface
}

// GetRemoteCertificate retrieves a certificate issued in the past,
// given the clusterid and the signingRequest.
func (identityProvider *certificateIdentityProvider) GetRemoteCertificate(clusterID,
	signingRequest string) (response responsetypes.SigningRequestResponse, err error) {
	namespace, err := identityProvider.namespaceManager.GetNamespace(clusterID)
	if err != nil {
		klog.Error(err)
		return response, err
	}

	secret, err := identityProvider.client.CoreV1().Secrets(namespace.Name).Get(context.TODO(), remoteCertificateSecret, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			klog.V(4).Info(err)
		} else {
			klog.Error(err)
		}
		return response, err
	}

	signingRequestSecret, ok := secret.Data[csrSecretKey]
	if !ok {
		klog.Errorf("no %v key in secret %v/%v", csrSecretKey, secret.Namespace, secret.Name)
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, remoteCertificateSecret)
		return response, err
	}

	// check that this certificate is related to this signing request
	if csr := base64.StdEncoding.EncodeToString(signingRequestSecret); csr != signingRequest {
		err = kerrors.NewBadRequest(fmt.Sprintf("the stored and the provided CSR for cluster %s does not match", clusterID))
		klog.Error(err)
		return response, err
	}

	response.ResponseType = responsetypes.SigningRequestResponseCertificate
	response.Certificate, ok = secret.Data[certificateSecretKey]
	if !ok {
		klog.Errorf("no %v key in secret %v/%v", certificateSecretKey, secret.Namespace, secret.Name)
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    "v1",
			Resource: "secrets",
		}, remoteCertificateSecret)
		return response, err
	}

	return response, nil
}

// ApproveSigningRequest approves a remote CertificateSigningRequest.
// It creates a CertificateSigningRequest CR to be issued by the local cluster, and approves it.
// This function will wait (with a timeout) for an available certificate before returning.
func (identityProvider *certificateIdentityProvider) ApproveSigningRequest(clusterID,
	signingRequest string) (response responsetypes.SigningRequestResponse, err error) {
	rnd := fmt.Sprintf("%v", rand.Int63())

	signingBytes, err := base64.StdEncoding.DecodeString(signingRequest)
	if err != nil {
		klog.Error(err)
		return response, err
	}

	cert := &certv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: strings.Join([]string{identitySecretRoot, ""}, "-"),
			Labels: map[string]string{
				// the informer needs to select it by label, this is a temporal ID for this request
				randomIDLabel: rnd,
			},
		},
		Spec: certv1.CertificateSigningRequestSpec{
			Groups: []string{
				"system:authenticated",
			},
			SignerName: certv1.KubeAPIServerClientSignerName,
			Request:    signingBytes,
			Usages: []certv1.KeyUsage{
				certv1.UsageDigitalSignature,
				certv1.UsageKeyEncipherment,
				certv1.UsageClientAuth,
			},
		},
	}

	cert, err = identityProvider.client.CertificatesV1().CertificateSigningRequests().Create(context.TODO(), cert, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return response, err
	}

	// approve the CertificateSigningRequest
	if err = certificateSigningRequest.ApproveCSR(identityProvider.client, cert, "IdentityManagerApproval",
		"This CSR was approved by Liqo Identity Manager"); err != nil {
		klog.Error(err)
		return response, err
	}

	// retrieve the certificate issued by the Kubernetes issuer in the CSR
	response.ResponseType = responsetypes.SigningRequestResponseCertificate
	response.Certificate, err = identityProvider.getCertificate(cert, rnd)
	if err != nil {
		klog.Error(err)
		return response, err
	}

	// store the certificate in a Secret, in this way is possbile to retrieve it again in the future
	if _, err = identityProvider.storeRemoteCertificate(clusterID, signingBytes, response.Certificate); err != nil {
		klog.Error(err)
		return response, err
	}
	return response, nil
}

// getCertificate retrieves the certificate given the CertificateSigningRequest and its randomID.
// If the certificate is not ready yet, it will wait for it (with a timeout).
func (identityProvider *certificateIdentityProvider) getCertificate(csr *certv1.CertificateSigningRequest, randomID string) ([]byte, error) {
	var certificate []byte

	// define a function that will check if a generic object is a CSR with a issued certificate
	checkCertificate := func(obj interface{}) bool {
		certificateSigningRequest, ok := obj.(*certv1.CertificateSigningRequest)
		if !ok {
			klog.Errorf("this object is not a CertificateSigningRequest: %v", obj)
			return false
		}

		res := certificateSigningRequest.Status.Certificate != nil && len(certificateSigningRequest.Status.Certificate) > 0
		if res {
			certificate = certificateSigningRequest.Status.Certificate
		}
		return res
	}

	if checkCertificate(csr) {
		// the csr is already valid, don't wait for the certificate
		return csr.Status.Certificate, nil
	}

	// create an informer to be notified when the certificate will be available
	// this informer will only watch one CSR, thanks to the random ID
	labelSelector := labels.Set(map[string]string{
		randomIDLabel: randomID,
	}).AsSelector()

	informer := cache.NewSharedIndexInformer(&cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.LabelSelector = labelSelector.String()
			return identityProvider.client.CertificatesV1().CertificateSigningRequests().List(context.TODO(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = labelSelector.String()
			return identityProvider.client.CertificatesV1().CertificateSigningRequests().Watch(context.TODO(), options)
		},
	}, &certv1.CertificateSigningRequest{}, 0, cache.Indexers{})

	stopChan := make(chan struct{})
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if checkCertificate(obj) {
				close(stopChan)
			}
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			if checkCertificate(newObj) {
				close(stopChan)
			}
		},
	})

	go informer.Run(stopChan)

	// wait for the certificate, with a timeout
	select {
	case <-stopChan:
		// finished successfully
		return certificate, nil
	case <-time.NewTimer(30 * time.Second).C:
		err := fmt.Errorf("timeout exceeded waiting for the approved certificate")
		klog.Error(err)
		close(stopChan)
		return nil, err
	}
}

// storeRemoteCertificate stores the issued certificate in a Secret in the TenantNamespace.
func (identityProvider *certificateIdentityProvider) storeRemoteCertificate(
	clusterID string, signingRequest, certificate []byte) (*v1.Secret, error) {
	namespace, err := identityProvider.namespaceManager.GetNamespace(clusterID)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      remoteCertificateSecret,
			Namespace: namespace.Name,
		},
		Data: map[string][]byte{
			csrSecretKey:         signingRequest,
			certificateSecretKey: certificate,
		},
	}

	if secret, err = identityProvider.client.CoreV1().
		Secrets(namespace.Name).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		klog.Error(err)
		return nil, err
	}
	return secret, nil
}
