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

package authservice

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/julienschmidt/httprouter"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"

	"github.com/liqotech/liqo/pkg/auth"
	autherrors "github.com/liqotech/liqo/pkg/auth/errors"
	authenticationtoken "github.com/liqotech/liqo/pkg/utils/authenticationtoken"
	traceutils "github.com/liqotech/liqo/pkg/utils/trace"
)

// identity handles the certificate identity http request.
func (authService *Controller) identity(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tracer := trace.New("Identity handler")
	ctx := trace.ContextWithTrace(r.Context(), tracer)
	defer tracer.LogIfLong(traceutils.LongThreshold())

	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	identityRequest := auth.CertificateIdentityRequest{}
	err = json.Unmarshal(bytes, &identityRequest)
	if err != nil {
		klog.Error(err)
		err = &autherrors.ClientError{
			Reason: err.Error(),
		}
		authService.handleError(w, err)
		return
	}

	response, err := authService.handleIdentity(ctx, identityRequest)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}
	klog.V(8).Infof("Sending response: %v", response)

	respBytes, err := json.Marshal(response)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	if _, err = w.Write(respBytes); err != nil {
		klog.Error(err)
		return
	}
}

// handleIdentity creates a certificate and a CertificateIdentityResponse, given a CertificateIdentityRequest.
func (authService *Controller) handleIdentity(
	ctx context.Context, identityRequest auth.CertificateIdentityRequest) (*auth.CertificateIdentityResponse, error) {
	tracer := trace.FromContext(ctx).Nest("Identity handling")
	defer tracer.LogIfLong(traceutils.LongThreshold())
	var err error

	// check that the provided credentials are valid
	klog.V(4).Info("Checking credentials")
	if err = authService.credentialsValidator.checkCredentials(
		&identityRequest, authService.getTokenManager(), authService.authenticationEnabled); err != nil {
		klog.Error(err)
		return nil, err
	}
	tracer.Step("Credentials checked")

	klog.V(4).Infof("Creating Tenant Namespace for cluster %v", identityRequest.GetClusterID())
	namespace, err := authService.namespaceManager.CreateNamespace(identityRequest.GetClusterID())
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	tracer.Step("Tenant namespace created")

	// check that there is no available certificate for that clusterID
	if _, err = authService.identityProvider.GetRemoteCertificate(
		identityRequest.ClusterID, namespace.Name, identityRequest.CertificateSigningRequest); err == nil {
		klog.Info("multiple identity validations with unique clusterID")
		err = &kerrors.StatusError{ErrStatus: metav1.Status{
			Status: metav1.StatusFailure,
			Code:   http.StatusForbidden,
			Reason: metav1.StatusReasonForbidden,
		}}
		klog.Error(err)
		return nil, err
	} else if !kerrors.IsNotFound(err) {
		klog.Error(err)
		return nil, err
	}
	tracer.Step("Cluster ID uniqueness ensured")

	// issue certificate request
	identityResponse, err := authService.identityProvider.ApproveSigningRequest(
		identityRequest.ClusterID, identityRequest.CertificateSigningRequest)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	tracer.Step("Certificate signing request approved")

	// bind basic permission required to start the peering
	if _, err = authService.namespaceManager.BindClusterRoles(
		identityRequest.GetClusterID(), authService.peeringPermission.Basic...); err != nil {
		klog.Error(err)
		return nil, err
	}
	tracer.Step("Cluster roles bound")

	// make the response to send to the remote cluster
	response, err := auth.NewCertificateIdentityResponse(namespace.Name, identityResponse, authService.apiServerConfig)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	tracer.Step("Identity response prepared")

	if identityRequest.OriginClusterToken != "" {
		// store the retrieved token
		err = authenticationtoken.StoreInSecret(ctx, authService.clientset,
			identityRequest.ClusterID, identityRequest.OriginClusterToken, authService.namespace)
		if err != nil {
			klog.Error(err)
			return nil, err
		}
		tracer.Step("Origin cluster token stored")
	}

	klog.Infof("Identity Request successfully validated for cluster %v", identityRequest.GetClusterID())
	return response, nil
}
