// Copyright 2019-2022 The Liqo Authors
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

package common

import (
	"context"
	"fmt"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

type fcEventType string
type fcEventChecker func(fc *discoveryv1alpha1.ForeignCluster) bool

const (
	// UnpeeringEvent name of the unpeering event.
	UnpeeringEvent fcEventType = "unpeer"
	// AuthEvent name of the authentication event.
	AuthEvent fcEventType = "authentication"
)

var (
	// UnpeerChecker checks if the two clusters are unpeered.
	UnpeerChecker fcEventChecker = func(fc *discoveryv1alpha1.ForeignCluster) bool {
		return foreigncluster.IsIncomingPeeringNone(fc) && foreigncluster.IsOutgoingPeeringNone(fc)
	}

	// AuthChecker checks if the authentication has been completed.
	AuthChecker fcEventChecker = foreigncluster.IsAuthenticated
)

// WaitForEventOnForeignCluster given the remote cluster identity if waits for the given event
// to be verified on the associated foreigncluster.
func WaitForEventOnForeignCluster(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity,
	event fcEventType, checker fcEventChecker, timeout time.Duration, cl client.Client) error {
	deadLine := time.After(timeout)
	remName := remoteClusterID.ClusterName
	remID := remoteClusterID.ClusterID
	for {
		select {
		case <-deadLine:
			return fmt.Errorf("timout (%.0fs) expired while waiting for event {%s} from cluster {%s}",
				timeout.Seconds(), event, remName)
		default:
			fc, err := foreigncluster.GetForeignClusterByID(ctx, cl, remID)
			if err != nil && !k8serrors.IsNotFound(err) {
				return err
			} else if k8serrors.IsNotFound(err) {
				return nil
			}
			if checker(fc) {
				return nil
			}
			time.Sleep(2 * time.Second)
		}
	}
}
