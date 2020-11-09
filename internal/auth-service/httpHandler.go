package auth_service

import (
	"encoding/json"
	"github.com/julienschmidt/httprouter"
	"github.com/liqotech/liqo/pkg/auth"
	"io/ioutil"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"net/http"
)

func (authService *AuthServiceCtrl) role(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	roleRequest := &auth.RoleRequest{}
	err = json.Unmarshal(bytes, roleRequest)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	if token, err := authService.getToken(); err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	} else if token != roleRequest.Token {
		err = &kerrors.StatusError{ErrStatus: metav1.Status{
			Status: metav1.StatusFailure,
			Code:   http.StatusForbidden,
			Reason: metav1.StatusReasonForbidden,
		}}
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	sa, err := authService.createServiceAccount(roleRequest.ClusterID)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	role, err := authService.createRole(roleRequest.ClusterID, sa)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	_, err = authService.createRoleBinding(roleRequest.ClusterID, sa, role)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	clusterRole, err := authService.createClusterRole(roleRequest.ClusterID, sa)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	_, err = authService.createClusterRoleBinding(roleRequest.ClusterID, sa, clusterRole)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	sa, err = authService.getServiceAccountCompleted(roleRequest.ClusterID)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	kubeconfig, err := authService.createKubeConfig(sa)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte(kubeconfig))
	if err != nil {
		klog.Error(err)
		return
	}
}

func (authService *AuthServiceCtrl) handleError(w http.ResponseWriter, err error) {
	// TODO: switch on error type
	authService.sendError(w, err.Error(), http.StatusInternalServerError)
}

func (authService *AuthServiceCtrl) sendError(w http.ResponseWriter, resp interface{}, code int) {
	bytes := []byte{}
	var err error
	if resp != nil {
		bytes, err = json.Marshal(resp)
		if err != nil {
			klog.Error(err)
			return
		}
	}
	http.Error(w, string(bytes), code)
}
