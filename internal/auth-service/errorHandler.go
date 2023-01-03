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
	"net/http"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	autherrors "github.com/liqotech/liqo/pkg/auth/errors"
)

func (authService *Controller) handleError(w http.ResponseWriter, err error) {
	switch err.(type) {
	case *kerrors.StatusError:
		authService.sendError(w, err.Error(), http.StatusForbidden)
	case *autherrors.ClientError:
		authService.sendError(w, err.Error(), http.StatusBadRequest)
	case *autherrors.AuthenticationFailedError:
		authService.sendError(w, err.Error(), http.StatusUnauthorized)
	default:
		authService.sendError(w, err.Error(), http.StatusInternalServerError)
	}
}

func (authService *Controller) sendError(w http.ResponseWriter, resp string, code int) {
	klog.V(3).Infof("%v - sending error response: %v", code, resp)
	http.Error(w, resp, code)
}
