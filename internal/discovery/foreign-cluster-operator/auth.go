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

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/discovery"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
)

const (
	identityAcceptedReason  = "IdentityAccepted"
	identityAcceptedMessage = "The Identity has been correctly accepted by the remote cluster"

	identityDeniedEmptyTokenReason  = "IdentityEmptyDenied"
	identityDeniedEmptyTokenMessage = "The remote cluster denied the empty token authentication method."

	identityDeniedReason  = "IdentityDenied"
	identityDeniedMessage = "The remote cluster denied the authentication token provided."
)

// ensureRemoteIdentity tries to fetch the remote identity from the secret, if it is not found
// it creates a new identity and sends it to the remote cluster.
func (r *ForeignClusterReconciler) ensureRemoteIdentity(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	_, err := r.identityManager.GetConfig(foreignCluster.Spec.ClusterIdentity.ClusterID, foreignCluster.Status.TenantNamespace.Local)
	if err != nil && !kerrors.IsNotFound(err) {
		klog.Error(err)
		return err
	}
	if err == nil {
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.AuthenticationStatusCondition,
			discoveryv1alpha1.PeeringConditionStatusEstablished,
			identityAcceptedReason,
			identityAcceptedMessage)
	} else {
		_, err = r.validateIdentity(foreignCluster)
		if err != nil {
			klog.Error(err)
			return err
		}
	}

	return nil
}

// fetchRemoteTenantNamespace fetches the remote tenant namespace name form the local identity secret
// and loads it in the ForeignCluster.
func (r *ForeignClusterReconciler) fetchRemoteTenantNamespace(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	remoteNamespace, err := r.identityManager.GetRemoteTenantNamespace(
		foreignCluster.Spec.ClusterIdentity.ClusterID, foreignCluster.Status.TenantNamespace.Local)
	if err != nil {
		klog.Error(err)
		return err
	}

	foreignCluster.Status.TenantNamespace.Remote = remoteNamespace
	return nil
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

// validateIdentity sends an HTTP request to validate the identity for the remote cluster (Certificate).
func (r *ForeignClusterReconciler) validateIdentity(fc *discoveryv1alpha1.ForeignCluster) (
	authStatus discoveryv1alpha1.PeeringConditionStatusType, err error) {
	remoteClusterID := fc.Spec.ClusterIdentity.ClusterID
	token := r.getAuthToken(fc)

	_, err = r.identityManager.CreateIdentity(remoteClusterID)
	if err != nil {
		klog.Error(err)
		return discoveryv1alpha1.PeeringConditionStatusPending, err
	}

	csr, err := r.identityManager.GetSigningRequest(remoteClusterID)
	if err != nil {
		klog.Error(err)
		return discoveryv1alpha1.PeeringConditionStatusPending, err
	}

	request := auth.NewCertificateIdentityRequest(r.clusterID.GetClusterID(), token, csr)
	responseBytes, authStatus, err := sendIdentityRequest(request, fc)
	if err != nil {
		klog.Error(err)
		return discoveryv1alpha1.PeeringConditionStatusPending, err
	}

	response := auth.CertificateIdentityResponse{}
	if err = json.Unmarshal(responseBytes, &response); err != nil {
		klog.Error(err)
		return discoveryv1alpha1.PeeringConditionStatusPending, err
	}

	if err = r.identityManager.StoreCertificate(remoteClusterID, &response); err != nil {
		klog.Error(err)
		return discoveryv1alpha1.PeeringConditionStatusPending, err
	}

	return authStatus, nil
}

// sendIdentityRequest sends an HTTP request to the remote cluster.
func sendIdentityRequest(request auth.IdentityRequest, fc *discoveryv1alpha1.ForeignCluster) (
	[]byte, discoveryv1alpha1.PeeringConditionStatusType, error) {
	jsonRequest, err := json.Marshal(request)
	if err != nil {
		klog.Error(err)
		return nil, discoveryv1alpha1.PeeringConditionStatusPending, err
	}
	klog.V(4).Infof("[%v] Sending json request: %v", fc.Spec.ClusterIdentity.ClusterID, string(jsonRequest))

	resp, err := sendRequest(
		fmt.Sprintf("%s%s", fc.Spec.ForeignAuthURL, request.GetPath()),
		bytes.NewBuffer(jsonRequest),
		foreignclusterutils.InsecureSkipTLSVerify(fc))
	if err != nil {
		klog.Error(err)
		return nil, discoveryv1alpha1.PeeringConditionStatusPending, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.Error(err)
		return nil, discoveryv1alpha1.PeeringConditionStatusPending, err
	}

	var authStatus discoveryv1alpha1.PeeringConditionStatusType
	var reason, message string
	defer func() {
		peeringconditionsutils.EnsureStatus(fc, discoveryv1alpha1.AuthenticationStatusCondition, authStatus, reason, message)
	}()

	switch resp.StatusCode {
	case http.StatusAccepted, http.StatusCreated:
		authStatus = discoveryv1alpha1.PeeringConditionStatusEstablished
		reason = identityAcceptedReason
		message = identityAcceptedMessage
		klog.V(8).Infof("[%v] Received body: %v", fc.Spec.ClusterIdentity.ClusterID, string(body))
		klog.V(4).Infof("[%v] Status Code: %v", fc.Spec.ClusterIdentity.ClusterID, resp.StatusCode)
		klog.Infof("[%v] Identity Accepted", fc.Spec.ClusterIdentity.ClusterID)
		return body, authStatus, nil
	case http.StatusForbidden:
		if request.GetToken() == "" {
			authStatus = discoveryv1alpha1.PeeringConditionStatusEmptyDenied
			reason = identityDeniedEmptyTokenReason
			message = identityDeniedEmptyTokenMessage
		} else {
			authStatus = discoveryv1alpha1.PeeringConditionStatusDenied
			reason = identityDeniedReason
			message = identityDeniedMessage
		}
		klog.Infof("[%v] Received body: %v", fc.Spec.ClusterIdentity.ClusterID, string(body))
		klog.Infof("[%v] Status Code: %v", fc.Spec.ClusterIdentity.ClusterID, resp.StatusCode)
		return nil, authStatus, nil
	default:
		authStatus = discoveryv1alpha1.PeeringConditionStatusPending
		klog.Infof("[%v] Received body: %v", fc.Spec.ClusterIdentity.ClusterID, string(body))
		klog.Infof("[%v] Status Code: %v", fc.Spec.ClusterIdentity.ClusterID, resp.StatusCode)
		return nil, authStatus, errors.New(string(body))
	}
}

func sendRequest(url string, payload *bytes.Buffer, insecureSkipTLSVerify bool) (*http.Response, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipTLSVerify},
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
