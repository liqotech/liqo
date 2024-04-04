// Copyright 2019-2024 The Liqo Authors
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

package modules

import (
	"context"
	"encoding/base64"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	identitycontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/identity-controller"
	noncecreatorcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/noncecreator-controller"
	noncesigner "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/noncesigner-controller"
	tenantcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/tenant-controller"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

// AuthOption defines the options to setup the authentication module.
type AuthOption struct {
	IdentityProvider         identitymanager.IdentityProvider
	NamespaceManager         tenantnamespace.Manager
	LiqoNamespace            string
	APIServerAddressOverride string
	CAOverrideB64            string
	TrustedCA                bool
}

// SetupAuthenticationModule setup the authentication module and initializes its controllers .
func SetupAuthenticationModule(ctx context.Context, mgr manager.Manager, uncachedClient client.Client,
	opts *AuthOption) error {
	var caOverride []byte
	if opts.CAOverrideB64 != "" {
		caOverride = make([]byte, base64.StdEncoding.DecodedLen(len(opts.CAOverrideB64)))
		_, err := base64.StdEncoding.Decode(caOverride, []byte(opts.CAOverrideB64))
		if err != nil {
			klog.Errorf("Unable to decode the CA override: %v", err)
			return err
		}
	}

	if err := enforceAuthenticationKeys(ctx, uncachedClient, opts.LiqoNamespace); err != nil {
		return err
	}

	// Configure controller that generate nonces.
	nonceReconciler := noncecreatorcontroller.NewNonceReconciler(
		mgr.GetClient(), mgr.GetScheme(),
		opts.NamespaceManager,
		mgr.GetEventRecorderFor("nonce-controller"),
	)
	if err := nonceReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to setup the nonce controller: %v", err)
		return err
	}

	// Configure controller that sign nonces with the private key of the cluster.
	nonceSignerReconciler := noncesigner.NewNonceSignerReconciler(mgr.GetClient(), mgr.GetScheme(),
		mgr.GetEventRecorderFor("signed-nonce-controller"),
		opts.NamespaceManager, opts.LiqoNamespace)
	if err := nonceSignerReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to setup the nonce signer reconciler: %v", err)
		return err
	}

	// Configure controller that fill tenant status with the authentication parameters.
	tenantReconciler := tenantcontroller.NewTenantReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(),
		mgr.GetEventRecorderFor("tenant-controller"),
		opts.IdentityProvider, opts.NamespaceManager,
		opts.APIServerAddressOverride, caOverride, opts.TrustedCA)
	if err := tenantReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to setup the tenant controller: %v", err)
		return err
	}

	// Configure controller that create Kubeconfig secrets for each identities.
	identityReconciler := identitycontroller.NewIdentityReconciler(mgr.GetClient(), mgr.GetScheme(),
		mgr.GetEventRecorderFor("identity-controller"), opts.LiqoNamespace)
	if err := identityReconciler.SetupWithManager(mgr); err != nil {
		klog.Errorf("Unable to setup the identity reconciler: %v", err)
		return err
	}

	return nil
}

func enforceAuthenticationKeys(ctx context.Context, cl client.Client, liqoNamespace string) error {
	if err := authentication.InitClusterKeys(ctx, cl, liqoNamespace); err != nil {
		klog.Errorf("Unable to initialize cluster authentication keys: %v", err)
	}

	klog.Info("Enforced cluster authentication keys")
	return nil
}
