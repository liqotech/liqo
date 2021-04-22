package auth_service

import (
	"github.com/liqotech/liqo/pkg/auth"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"net/http"
)

type credentialsValidator interface {
	checkCredentials(roleRequest auth.IdentityRequest, configProvider auth.AuthConfigProvider, tokenManager tokenManager) error

	validEmptyToken(configProvider auth.AuthConfigProvider) bool
	validToken(tokenManager tokenManager, token string) (bool, error)
}

type tokenValidator struct{}

// checkCredentials checks if the provided token is valid for the local cluster given an IdentityRequest
func (tokenValidator *tokenValidator) checkCredentials(roleRequest auth.IdentityRequest, configProvider auth.AuthConfigProvider, tokenManager tokenManager) error {
	// token check fails if:
	// 1. token is different from the correct one
	// 2. token is empty but in the cluster config empty token is not allowed

	if tokenValidator.validEmptyToken(configProvider) {
		return nil
	}
	if valid, err := tokenValidator.validToken(tokenManager, roleRequest.GetToken()); err != nil {
		klog.Error(err)
		return err
	} else if valid {
		return nil
	} else {
		err = &kerrors.StatusError{ErrStatus: metav1.Status{
			Status: metav1.StatusFailure,
			Code:   http.StatusForbidden,
			Reason: metav1.StatusReasonForbidden,
		}}
		klog.Error(err)
		return err
	}
}

// validEmptyToken checks if the empty token is accepted
func (tokenValidator *tokenValidator) validEmptyToken(configProvider auth.AuthConfigProvider) bool {
	return configProvider.GetConfig().AllowEmptyToken
}

// validToken checks if the token provided is valid
func (tokenValidator *tokenValidator) validToken(tokenManager tokenManager, token string) (bool, error) {
	correctToken, err := tokenManager.getToken()
	if err != nil {
		klog.Error(err)
		return false, err
	}

	return token == correctToken, nil
}
