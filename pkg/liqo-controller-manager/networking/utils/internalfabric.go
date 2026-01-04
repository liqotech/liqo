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

package utils //nolint:revive // we want to use this package name

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// ForgeInternalFabricName retrieves the name of the InternalFabric resource associated with the given metadata.
// WARNING: this function try to check if an InternalFabric resource already
// exists with the name of the gateway resource.
// This is intended to avoid breaking changes, since the InternalFabric name
// has changed from Gateway's name to the Gateway's remote cluster ID.
func ForgeInternalFabricName(ctx context.Context, cl client.Client, meta *metav1.ObjectMeta) (string, error) {
	internalFabric := &networkingv1beta1.InternalFabric{}

	err := cl.Get(ctx, client.ObjectKey{
		Name:      meta.Name,
		Namespace: meta.Namespace,
	}, internalFabric)

	switch {
	case apierrors.IsNotFound(err):
		remoteClusterID, err := getters.RetrieveRemoteClusterIDFromMeta(meta)
		if err != nil {
			return "", fmt.Errorf("unable to retrieve remote cluster ID from object %s/%s:%w",
				meta.GetNamespace(), meta.GetName(), err)
		}
		return string(remoteClusterID), nil
	case err != nil:
		return "", fmt.Errorf("unable to get the internal fabric %q: %w", meta.Name, err)
	}
	return internalFabric.Name, nil
}
