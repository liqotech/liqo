// Copyright 2019-2025 The Liqo Authors
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

package geneve

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// getInternalFabric retrieves the InternalFabric resource associated with the given GatewayServer.
// WARNING: this function contains 2 calls to the Kubernetes API using 2 different names.
// This is intended to avoid breaking changes, since the InternalFabric name has changed from GatewayServer name to the GatewayServer cluster ID.
func getInternalFabric(ctx context.Context, cl client.Client, gatewayName, remoteID, ns string) (*networkingv1beta1.InternalFabric, error) {
	internalFabric := &networkingv1beta1.InternalFabric{}
	err := cl.Get(ctx, client.ObjectKey{
		Name:      remoteID,
		Namespace: ns,
	}, internalFabric)
	if client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("unable to get the internal fabric %q: %w", remoteID, err)
	}

	if err == nil {
		return internalFabric, nil
	}

	err = cl.Get(ctx, client.ObjectKey{
		Name:      gatewayName,
		Namespace: ns,
	}, internalFabric)

	switch {
	case errors.IsNotFound(err):
		return nil, fmt.Errorf("could not find internal fabric with names %q and %q: %w", gatewayName, remoteID, err)
	case err != nil:
		return nil, fmt.Errorf("unable to get the internal fabric %q: %w", gatewayName, err)
	}

	return internalFabric, nil
}
