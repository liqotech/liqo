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

package wireguard

import (
	"context"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cluster-role
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;delete;update

// EnsureKeysSecret ensure the presence of the private and public keys for the Wireguard interface and save them inside a Secret resource and Options.
func EnsureKeysSecret(ctx context.Context, cl client.Client, opts *Options) error {
	var pri, pub wgtypes.Key
	var err error
	pri, err = CheckKeysSecret(ctx, cl, opts)

	switch {
	case kerrors.IsNotFound(err) || len(pri) == 0:
		pri, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}
		pub = pri.PublicKey()
		if err := CreateKeysSecret(ctx, cl, opts, pri, pub); err != nil {
			return err
		}
	case err != nil:
		return err
	}

	opts.PrivateKey = pri

	return nil
}
