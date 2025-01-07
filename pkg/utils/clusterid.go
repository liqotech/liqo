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

package utils

import (
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// GetClusterIDFromLabels returns the clusterID from the given labels.
func GetClusterIDFromLabels(labels map[string]string) (liqov1beta1.ClusterID, bool) {
	return GetClusterIDFromLabelsWithKey(labels, consts.RemoteClusterID)
}

// GetClusterIDFromLabelsWithKey returns the clusterID from the given labels with the given key.
func GetClusterIDFromLabelsWithKey(labels map[string]string, key string) (liqov1beta1.ClusterID, bool) {
	if labels == nil {
		return "", false
	}
	tmp, ok := labels[key]
	if !ok {
		return "", false
	}
	return liqov1beta1.ClusterID(tmp), true
}
