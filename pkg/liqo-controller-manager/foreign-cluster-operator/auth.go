// Copyright 2019-2023 The Liqo Authors
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

package foreignclusteroperator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/discoverymanager/utils"
	"github.com/liqotech/liqo/pkg/utils/authenticationtoken"
	csrutil "github.com/liqotech/liqo/pkg/utils/csr"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
)

const (
	identityAcceptedReason  = "IdentityAccepted"
	identityAcceptedMessage = "The Identity has been correctly accepted by the remote cluster"

	identityDeniedEmptyTokenReason  = "IdentityEmptyDenied"
	identityDeniedEmptyTokenMessage = "The remote cluster requires cluster authentication to be enabled: %v"

	identityDeniedReason  = "IdentityDenied"
	identityDeniedMessage = "Cluster authentication denied by the remote cluster: %v"

	identityErrorReason  = "IdentityError"
	identityErrorMessage = "Cannot ensure identity: %v"
)

type identityDeniedError struct{ msg string }

func (err identityDeniedError) Error() string { return err.msg }

type identityEmptyDeniedError struct{ msg string }

func (err identityEmptyDeniedError) Error() string { return err.msg }

// ensureRemoteIdentity tries to fetch the remote identity from the secret, if it is not found
// it creates a new identity and sends it to the remote cluster.
func (r *ForeignClusterReconciler) ensureRemoteIdentity(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) (err error) {
	status := discoveryv1alpha1.PeeringConditionStatusError
	var reason, message string
	defer func() {
		if status == discoveryv1alpha1.PeeringConditionStatusError {
			reason = identityErrorReason
			message = fmt.Sprintf(identityErrorMessage, err)
		}
		peeringconditionsutils.EnsureStatus(foreignCluster, discoveryv1alpha1.AuthenticationStatusCondition, status, reason, message)
	}()

	_, err = r.IdentityManager.GetConfig(foreignCluster.Spec.ClusterIdentity, foreignCluster.Status.TenantNamespace.Local)
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}

	if err != nil {
		if err = r.validateIdentity(ctx, foreignCluster); err != nil {
			if errors.Is(err, identityEmptyDeniedError{}) {
				status = discoveryv1alpha1.PeeringConditionStatusEmptyDenied
				reason = identityDeniedEmptyTokenReason
				message = fmt.Sprintf(identityDeniedEmptyTokenMessage, err)
			} else if errors.Is(err, identityDeniedError{}) {
				status = discoveryv1alpha1.PeeringConditionStatusDenied
				reason = identityDeniedReason
				message = fmt.Sprintf(identityDeniedMessage, err)
			}
			return err
		}
	}

	status = discoveryv1alpha1.PeeringConditionStatusEstablished
	reason = identityAcceptedReason
	message = identityAcceptedMessage

	return nil
}

// fetchRemoteTenantNamespace fetches the remote tenant namespace name form the local identity secret
// and loads it in the ForeignCluster.
func (r *ForeignClusterReconciler) fetchRemoteTenantNamespace(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	remoteNamespace, err := r.IdentityManager.GetRemoteTenantNamespace(
		foreignCluster.Spec.ClusterIdentity, foreignCluster.Status.TenantNamespace.Local)
	if err != nil {
		klog.Error(err)
		return err
	}

	foreignCluster.Status.TenantNamespace.Remote = remoteNamespace
	return nil
}

// validateIdentity sends an HTTP request to validate the identity for the remote cluster (Certificate).
func (r *ForeignClusterReconciler) validateIdentity(ctx context.Context, fc *discoveryv1alpha1.ForeignCluster) error {
	remoteCluster := fc.Spec.ClusterIdentity
	token, err := authenticationtoken.GetAuthToken(ctx, remoteCluster.ClusterID, r.Client)
	if err != nil {
		return err
	}

	key, csr, err := csrutil.NewKeyAndRequest(r.HomeCluster.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to create create identity: %w", err)
	}

	localToken, err := auth.GetToken(ctx, r.Client, r.LiqoNamespace)
	if err != nil {
		return fmt.Errorf("failed to retrieve authentication token: %w", err)
	}

	request := auth.NewCertificateIdentityRequest(r.HomeCluster, localToken, token, csr)
	responseBytes, err := r.sendIdentityRequest(ctx, request, fc)
	if err != nil {
		return fmt.Errorf("failed to send identity request: %w", err)
	}

	response := auth.CertificateIdentityResponse{}
	if err = json.Unmarshal(responseBytes, &response); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if err = r.IdentityManager.StoreIdentity(ctx, remoteCluster, fc.Status.TenantNamespace.Local,
		key, fc.Spec.ForeignProxyURL, &response); err != nil {
		return fmt.Errorf("failed to store identity: %w", err)
	}

	return nil
}

// sendIdentityRequest sends an HTTP request to the remote cluster.
func (r *ForeignClusterReconciler) sendIdentityRequest(ctx context.Context, request auth.IdentityRequest, fc *discoveryv1alpha1.ForeignCluster) (
	[]byte, error) {
	jsonRequest, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to mashal request: %w", err)
	}
	klog.V(8).Infof("[%v] Sending json request: %v", fc.Spec.ClusterIdentity.ClusterID, string(jsonRequest))

	resp, err := sendRequest(ctx,
		r.transport(foreignclusterutils.InsecureSkipTLSVerify(fc)),
		fmt.Sprintf("%s%s", fc.Spec.ForeignAuthURL, request.GetPath()),
		bytes.NewBuffer(jsonRequest))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusAccepted, http.StatusCreated:
		klog.V(8).Infof("[%v] Received body: %v", fc.Spec.ClusterIdentity, string(body))
		klog.V(4).Infof("[%v] Status Code: %v", fc.Spec.ClusterIdentity, resp.StatusCode)
		klog.Infof("[%v] Identity Accepted", fc.Spec.ClusterIdentity)
		return body, nil
	case http.StatusForbidden, http.StatusUnauthorized:
		klog.Infof("[%v] Received body: %v", fc.Spec.ClusterIdentity, string(body))
		klog.Infof("[%v] Status Code: %v", fc.Spec.ClusterIdentity, resp.StatusCode)
		if request.GetToken() == "" {
			return nil, identityEmptyDeniedError{string(body)}
		}
		return nil, identityDeniedError{string(body)}
	default:
		klog.Infof("[%v] Received body: %v", fc.Spec.ClusterIdentity.ClusterID, string(body))
		klog.Infof("[%v] Status Code: %v", fc.Spec.ClusterIdentity.ClusterID, resp.StatusCode)
		return nil, fmt.Errorf("unexpected response: %v", string(body))
	}
}

func sendRequest(ctx context.Context, transport *http.Transport, url string, payload *bytes.Buffer) (*http.Response, error) {
	client := &http.Client{
		Transport: transport,
		Timeout:   utils.HTTPRequestTimeout,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, payload)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain")
	return client.Do(req)
}

// transport returns the correct transport to be used for a given request.
func (r *ForeignClusterReconciler) transport(insecureSkipTLSVerify bool) *http.Transport {
	if insecureSkipTLSVerify {
		return r.InsecureTransport
	}

	return r.SecureTransport
}

// getAuthTokenSecretPredicate returns the predicate to select the secrets containing authentication tokens
// for the remote clusters.
func getAuthTokenSecretPredicate() predicate.Predicate {
	secretsPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      discovery.ClusterIDLabel,
				Operator: metav1.LabelSelectorOpExists,
			},
			{
				Key:      discovery.AuthTokenLabel,
				Operator: metav1.LabelSelectorOpExists,
			},
		},
	})

	utilruntime.Must(err)

	return secretsPredicate
}
