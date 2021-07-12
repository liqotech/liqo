package authservice

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/julienschmidt/httprouter"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/liqotech/liqo/pkg/auth"
)

// identity handles the certificate identity http request.
func (authService *Controller) identity(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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
		authService.handleError(w, err)
		return
	}

	response, err := authService.handleIdentity(identityRequest)
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
	identityRequest auth.CertificateIdentityRequest) (*auth.CertificateIdentityResponse, error) {
	var err error

	// check that the provided credentials are valid
	klog.V(4).Info("Checking credentials")
	if err = authService.credentialsValidator.checkCredentials(
		&identityRequest, authService.getConfigProvider(), authService.getTokenManager()); err != nil {
		klog.Error(err)
		return nil, err
	}

	klog.V(4).Infof("Creating Tenant Namespace for cluster %v", identityRequest.GetClusterID())
	namespace, err := authService.namespaceManager.CreateNamespace(identityRequest.GetClusterID())
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	// check that there is no available certificate for that clusterID
	if _, err = authService.identityManager.GetRemoteCertificate(
		identityRequest.ClusterID, identityRequest.CertificateSigningRequest); err == nil {
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

	// issue certificate request
	identityResponse, err := authService.identityManager.ApproveSigningRequest(
		identityRequest.ClusterID, identityRequest.CertificateSigningRequest)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	// bind basic permission required to start the peering
	if _, err = authService.namespaceManager.BindClusterRoles(
		identityRequest.GetClusterID(), authService.peeringPermission.Basic...); err != nil {
		klog.Error(err)
		return nil, err
	}

	// make the response to send to the remote cluster
	response, err := auth.NewCertificateIdentityResponse(
		namespace.Name, &identityResponse, authService.getConfigProvider(), authService.clientset, authService.restConfig)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	klog.Infof("Identity Request successfully validated for cluster %v", identityRequest.GetClusterID())
	return response, nil
}
