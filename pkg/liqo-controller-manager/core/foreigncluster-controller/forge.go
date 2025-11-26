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

package foreignclustercontroller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// ForgeForeignCluster creates a new ForeignCluster resource for the given cluster ID.
func ForgeForeignCluster(clusterID liqov1beta1.ClusterID) *liqov1beta1.ForeignCluster {
	return &liqov1beta1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(clusterID),
			Labels: map[string]string{
				consts.RemoteClusterID: string(clusterID),
			},
		},
		Spec: liqov1beta1.ForeignClusterSpec{
			ClusterID: clusterID,
		},
	}
}
