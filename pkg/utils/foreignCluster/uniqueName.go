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

package foreigncluster

import (
	"crypto/sha256"
	"encoding/hex"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// UniqueName returns a user-friendly name that is unique *in the context of the current cluster*.
// It depends on ClusterName, so the same cluster may have different UniqueNames in different clusters.
//
// Use it when reflecting resources on a remote cluster (see issue #966).
func UniqueName(cluster *discoveryv1alpha1.ClusterIdentity) string {
	// We add a unique suffix to the cluster name, built by taking part of the hash of the cluster ID.
	idHash := sha256.Sum256([]byte(cluster.ClusterID))
	// We want 6 chars, so we encode 3 bytes
	idHashHex := hex.EncodeToString(idHash[:3])
	return cluster.ClusterName + "-" + idHashHex
}
