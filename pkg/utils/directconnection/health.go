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

package directconnection

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// NotPeeredError reports that no Connection resource exists at all towards the listed
// clusters: the clusters were never network-peered.
type NotPeeredError struct {
	Clusters []string
}

func (e *NotPeeredError) Error() string {
	return fmt.Sprintf("no direct network peering to cluster(s) %v", e.Clusters)
}

// ErrConnectionsDown reports that Connections exist towards all the clusters, but at least
// one of them is not (yet) Connected (Connecting, Error): a transient state expected to
// recover on its own, needing no operator action.
var ErrConnectionsDown = errors.New("direct connection(s) not established")

// CheckConnections verifies that the direct connections towards all the given clusters are
// established, i.e. for every cluster ID a Connection resource labeled with that ID exists
// and its status is Connected. It returns:
//   - nil if every cluster has a Connection in Connected state;
//   - *NotPeeredError if one or more clusters have no Connection at all (misconfiguration);
//   - ErrConnectionsDown if all Connections exist but some are not Connected (transient);
//   - any other error if listing Connections fails.
//
// The check is fail-safe: callers use it to decide which EndpointSlice of a direct/indirect
// pair carries ready endpoints, and falling back to the indirect (hub-and-spoke) path is
// always safe, while routing to an unverified direct path is not.
func CheckConnections(ctx context.Context, cl client.Client, clusterIDs []string) error {
	if len(clusterIDs) == 0 {
		// no need to check any connection
		return nil
	}

	var connections networkingv1beta1.ConnectionList
	if err := cl.List(ctx, &connections); err != nil {
		return err
	}

	status := make(map[string]networkingv1beta1.ConnectionStatusValue, len(connections.Items))
	for i := range connections.Items {
		conn := &connections.Items[i]
		if remoteID := conn.Labels[consts.RemoteClusterID]; remoteID != "" {
			status[remoteID] = conn.Status.Value
		}
	}

	var missing []string
	down := false
	for _, clusterID := range clusterIDs {
		value, present := status[clusterID]
		switch {
		case !present:
			missing = append(missing, clusterID)
		case value != networkingv1beta1.Connected:
			down = true
		}
	}
	slices.Sort(missing)
	// The not-peered case takes precedence: it is the actionable one, and it implies the
	// direct path is down anyway, even if the other clusters are merely Connecting.
	switch {
	case len(missing) > 0:
		return &NotPeeredError{Clusters: missing}
	case down:
		return ErrConnectionsDown
	default:
		return nil
	}
}
