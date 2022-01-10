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

package installutils

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/utils"
)

// GetOldClusterName returns the cluster name used in the previous installation (if any).
func GetOldClusterName(ctx context.Context, k8sClient kubernetes.Interface) (string, error) {
	clusterName, err := utils.GetClusterName(ctx, k8sClient, LiqoNamespace)
	return clusterName, client.IgnoreNotFound(err)
}
