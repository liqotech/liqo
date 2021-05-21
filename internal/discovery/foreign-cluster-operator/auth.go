package foreignclusteroperator

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	client_scheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/kubeconfig"
)

// get a client to the remote cluster.
// if the ForeignCluster has a reference to the secret's role, load the configurations from that secret
// else try to get a role from the remote cluster.
//
// first of all, if the status is pending we can try to get a role with an empty token, if the remote cluster allows it
// the status will become accepted.
//
// if our status is EmptyRefused, this means that the remote cluster refused out request with the empty token,
// so we will wait to have a token to ask for the role again.
//
// while we are waiting for that secret this function will return no error, but an empty client.
func (r *ForeignClusterReconciler) getRemoteClient(
	fc *discoveryv1alpha1.ForeignCluster, gv *schema.GroupVersion) (*crdclient.CRDClient, error) {
	if client, err := r.getIdentity(fc, gv); err == nil {
		return client, nil
	} else if !kerrors.IsNotFound(err) {
		klog.Error(err)
		return nil, err
	}

	if fc.Status.AuthStatus == discovery.AuthStatusAccepted {
		// TODO: handle this possibility
		// this can happen if the role was accepted but the local secret has been removed
		err := errors.New("auth status is accepted but there is no secret found")
		klog.Error(err)
		return nil, err
	}

	// not existing role
	isPending := fc.Status.AuthStatus == discovery.AuthStatusPending || fc.Status.AuthStatus == ""
	isRefused := fc.Status.AuthStatus == discovery.AuthStatusEmptyRefused
	if isPending || (isRefused && r.getAuthToken(fc) != "") {

		kubeconfigBytes, err := r.askRemoteIdentity(fc)
		if err != nil {
			klog.Error(err)
			return nil, err
		}
		if kubeconfigBytes == nil {
			klog.V(4).Infof("[%v] Empty kubeconfig string", fc.Spec.ClusterIdentity.ClusterID)
			return nil, nil
		}
		klog.V(4).Infof("[%v] Creating kubeconfig", fc.Spec.ClusterIdentity.ClusterID)
		_, err = kubeconfig.CreateSecret(r.crdClient.Client(), r.Namespace, string(kubeconfigBytes), map[string]string{
			discovery.ClusterIDLabel:      fc.Spec.ClusterIdentity.ClusterID,
			discovery.RemoteIdentityLabel: "",
		})
		if err != nil {
			klog.Error(err)
			return nil, err
		}

		return nil, nil
	}

	klog.V(4).Infof("[%v] no available identity", fc.Spec.ClusterIdentity.ClusterID)
	return nil, nil
}

// ensureRemoteIdentity tries to fetch the remote identity from the secret, if it is not found
// it creates a new identity and sends it to the remote cluster.
func (r *ForeignClusterReconciler) ensureRemoteIdentity(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	_, err := r.identityManager.GetConfig(foreignCluster.Spec.ClusterIdentity.ClusterID, foreignCluster.Status.TenantControlNamespace.Local)
	if err != nil && !kerrors.IsNotFound(err) {
		klog.Error(err)
		return err
	}
	var authStatus discovery.AuthStatus
	if err == nil {
		authStatus = discovery.AuthStatusAccepted
	} else {
		authStatus, err = r.validateIdentity(foreignCluster)
		if err != nil {
			klog.Error(err)
			return err
		}
	}

	foreignCluster.Status.AuthStatus = authStatus
	return nil
}

// fetchRemoteTenantNamespace fetches the remote tenant namespace name form the local identity secret
// and loads it in the ForeignCluster.
func (r *ForeignClusterReconciler) fetchRemoteTenantNamespace(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	remoteNamespace, err := r.identityManager.GetRemoteTenantNamespace(
		foreignCluster.Spec.ClusterIdentity.ClusterID, foreignCluster.Status.TenantControlNamespace.Local)
	if err != nil {
		klog.Error(err)
		return err
	}

	foreignCluster.Status.TenantControlNamespace.Remote = remoteNamespace
	return nil
}

// getIdentity loads the remote identity from a secret.
func (r *ForeignClusterReconciler) getIdentity(
	fc *discoveryv1alpha1.ForeignCluster, gv *schema.GroupVersion) (*crdclient.CRDClient, error) {
	secrets, err := r.crdClient.Client().CoreV1().Secrets(r.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: strings.Join([]string{
			strings.Join([]string{discovery.ClusterIDLabel, fc.Spec.ClusterIdentity.ClusterID}, "="),
			discovery.RemoteIdentityLabel,
		}, ","),
	})
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	if len(secrets.Items) == 0 {
		err = kerrors.NewNotFound(schema.GroupResource{
			Group:    v1.GroupName,
			Resource: string(v1.ResourceSecrets),
		}, fmt.Sprintf("%s %s", discovery.RemoteIdentityLabel, fc.Spec.ClusterIdentity.ClusterID))
		return nil, err
	}

	roleSecret := &secrets.Items[0]

	config, err := kubeconfig.LoadFromSecret(roleSecret)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	config.ContentConfig.GroupVersion = gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = client_scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	fc.Status.AuthStatus = discovery.AuthStatusAccepted

	return crdclient.NewFromConfig(config)
}

// getAuthToken loads the auth token form a labeled secret.
func (r *ForeignClusterReconciler) getAuthToken(fc *discoveryv1alpha1.ForeignCluster) string {
	tokenSecrets, err := r.crdClient.Client().CoreV1().Secrets(r.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: strings.Join(
			[]string{
				strings.Join([]string{discovery.ClusterIDLabel, fc.Spec.ClusterIdentity.ClusterID}, "="),
				discovery.AuthTokenLabel,
			},
			",",
		),
	})
	if err != nil {
		klog.Error(err)
		return ""
	}

	for i := range tokenSecrets.Items {
		if token, found := tokenSecrets.Items[i].Data["token"]; found {
			return string(token)
		}
	}
	return ""
}

// askRemoteIdentity sends an HTTP request to get the identity from the remote cluster (ServiceAccount).
func (r *ForeignClusterReconciler) askRemoteIdentity(fc *discoveryv1alpha1.ForeignCluster) ([]byte, error) {
	token := r.getAuthToken(fc)

	roleRequest := auth.ServiceAccountIdentityRequest{
		ClusterID: r.clusterID.GetClusterID(),
		Token:     token,
	}
	resp, _, err := sendIdentityRequest(&roleRequest, fc)
	return resp, err
}

// validateIdentity sends an HTTP request to validate the identity for the remote cluster (Certificate).
func (r *ForeignClusterReconciler) validateIdentity(fc *discoveryv1alpha1.ForeignCluster) (authStatus discovery.AuthStatus, err error) {
	remoteClusterID := fc.Spec.ClusterIdentity.ClusterID
	token := r.getAuthToken(fc)

	_, err = r.identityManager.CreateIdentity(remoteClusterID)
	if err != nil {
		klog.Error(err)
		return discovery.AuthStatusPending, err
	}

	csr, err := r.identityManager.GetSigningRequest(remoteClusterID)
	if err != nil {
		klog.Error(err)
		return discovery.AuthStatusPending, err
	}

	request := auth.NewCertificateIdentityRequest(r.clusterID.GetClusterID(), token, csr)
	responseBytes, authStatus, err := sendIdentityRequest(request, fc)
	if err != nil {
		klog.Error(err)
		return discovery.AuthStatusPending, err
	}

	response := auth.CertificateIdentityResponse{}
	if err = json.Unmarshal(responseBytes, &response); err != nil {
		klog.Error(err)
		return discovery.AuthStatusPending, err
	}

	if err = r.identityManager.StoreCertificate(remoteClusterID, response); err != nil {
		klog.Error(err)
		return discovery.AuthStatusPending, err
	}

	return authStatus, nil
}

// sendIdentityRequest sends an HTTP request to the remote cluster.
func sendIdentityRequest(request auth.IdentityRequest, fc *discoveryv1alpha1.ForeignCluster) ([]byte, discovery.AuthStatus, error) {
	jsonRequest, err := json.Marshal(request)
	if err != nil {
		klog.Error(err)
		return nil, discovery.AuthStatusPending, err
	}
	klog.V(4).Infof("[%v] Sending json request: %v", fc.Spec.ClusterIdentity.ClusterID, string(jsonRequest))

	resp, err := sendRequest(
		fmt.Sprintf("%s%s", fc.Spec.AuthURL, request.GetPath()),
		bytes.NewBuffer(jsonRequest),
		fc.Spec.TrustMode == discovery.TrustModeTrusted)
	if err != nil {
		klog.Error(err)
		return nil, discovery.AuthStatusPending, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.Error(err)
		return nil, discovery.AuthStatusPending, err
	}
	switch resp.StatusCode {
	case http.StatusAccepted:
		fc.Status.AuthStatus = discovery.AuthStatusAccepted
		klog.V(8).Infof("[%v] Received body: %v", fc.Spec.ClusterIdentity.ClusterID, string(body))
		klog.V(4).Infof("[%v] Status Code: %v", fc.Spec.ClusterIdentity.ClusterID, resp.StatusCode)
		klog.Infof("[%v] Identity Accepted", fc.Spec.ClusterIdentity.ClusterID)
		return body, discovery.AuthStatusAccepted, nil
	case http.StatusCreated:
		fc.Status.AuthStatus = discovery.AuthStatusAccepted
		klog.V(8).Infof("[%v] Received body: %v", fc.Spec.ClusterIdentity.ClusterID, string(body))
		klog.V(4).Infof("[%v] Status Code: %v", fc.Spec.ClusterIdentity.ClusterID, resp.StatusCode)
		klog.Infof("[%v] Identity Created", fc.Spec.ClusterIdentity.ClusterID)
		return body, discovery.AuthStatusAccepted, nil
	case http.StatusForbidden:
		var authStatus discovery.AuthStatus
		if request.GetToken() == "" {
			authStatus = discovery.AuthStatusEmptyRefused
		} else {
			authStatus = discovery.AuthStatusRefused
		}
		fc.Status.AuthStatus = authStatus
		klog.Infof("[%v] Received body: %v", fc.Spec.ClusterIdentity.ClusterID, string(body))
		klog.Infof("[%v] Status Code: %v", fc.Spec.ClusterIdentity.ClusterID, resp.StatusCode)
		return nil, authStatus, nil
	default:
		klog.Infof("[%v] Received body: %v", fc.Spec.ClusterIdentity.ClusterID, string(body))
		klog.Infof("[%v] Status Code: %v", fc.Spec.ClusterIdentity.ClusterID, resp.StatusCode)
		return nil, discovery.AuthStatusPending, errors.New(string(body))
	}
}

func sendRequest(url string, payload *bytes.Buffer, isTrusted bool) (*http.Response, error) {
	tr := &http.Transport{}
	if !isTrusted {
		// disable TLS CA check for untrusted remote clusters
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodPost, url, payload)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain")
	return client.Do(req)
}
