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

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
)

// SetupAuthenticationModule setup the authentication module and initializes its controllers .
func SetupAuthenticationModule(ctx context.Context, uncachedClient client.Client, liqoNamespace string) error {
	if err := enforceAuthenticationKeys(ctx, uncachedClient, liqoNamespace); err != nil {
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
