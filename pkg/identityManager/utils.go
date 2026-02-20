// Copyright 2019-2026 The Liqo Authors
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

package identitymanager

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
)

// EnsureCertificate ensures that the certificate is present with the identity provider.
// When IsUpdate is true, it skips the cache entirely and directly approves a new signing request,
// which is used to force certificate renewal when the provider detects an expiring certificate.
func EnsureCertificate(ctx context.Context, idp IdentityProvider, options *SigningRequestOptions) (*responsetypes.SigningRequestResponse, error) {
	if options.IsRenew {
		// Skip cache entirely, directly approve a new signing request.
		resp, err := idp.ApproveSigningRequest(ctx, options)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}

	resp, err := idp.GetRemoteCertificate(ctx, options)
	switch {
	case apierrors.IsNotFound(err):
		// Certificate not found or CSR changed — generate new cert.
		resp, err = idp.ApproveSigningRequest(ctx, options)
		if err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	}

	return resp, nil
}
