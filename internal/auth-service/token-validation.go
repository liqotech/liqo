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

package authservice

import (
	"fmt"

	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/auth"
	autherrors "github.com/liqotech/liqo/pkg/auth/errors"
)

type credentialsValidator interface {
	checkCredentials(roleRequest auth.IdentityRequest, tokenManager tokenManager, authenticationEnabled bool) error
	validToken(tokenManager tokenManager, token string) (bool, error)
}

type tokenValidator struct{}

// checkCredentials checks if the provided token is valid for the local cluster given an IdentityRequest.
func (tokenValidator *tokenValidator) checkCredentials(
	roleRequest auth.IdentityRequest, tokenManager tokenManager, authenticationEnabled bool) error {
	// token check fails if the token is different from the correct one
	// and the authentication is disabled

	if !authenticationEnabled {
		klog.V(3).Infof("[%s] accepting credentials since authentication is disabled",
			roleRequest.GetClusterIdentity())
		return nil
	}

	valid, err := tokenValidator.validToken(tokenManager, roleRequest.GetToken())
	if err != nil {
		klog.Error(err)
		return err
	}
	if !valid {
		err = &autherrors.AuthenticationFailedError{
			Reason: fmt.Sprintf("invalid token %q", roleRequest.GetToken()),
		}
		klog.Error(err)
		return err
	}
	return nil
}

// validToken checks if the token provided is valid.
func (tokenValidator *tokenValidator) validToken(tokenManager tokenManager, token string) (bool, error) {
	correctToken, err := tokenManager.getToken()
	if err != nil {
		klog.Error(err)
		return false, err
	}

	return token == correctToken, nil
}
