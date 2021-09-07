// Copyright 2019-2021 The Liqo Authors
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

package consts

const (
	// ClusterIDConfigMapName is the name of the configmap where the cluster-id is stored.
	ClusterIDConfigMapName = "cluster-id"
	// MasterLabel contains the label used to identify the master nodes.
	MasterLabel = "node-role.kubernetes.io/master"
	// ServiceAccountNamespacePath contains the path where the namespace is stored in the serviceaccount volume mount.
	ServiceAccountNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	// ClusterIDLabelName is the name of the label key to use with Cluster ID.
	ClusterIDLabelName = "clusterID"
	// ClusterIDConfigMapKey is the key of the configmap where the cluster-id is stored.
	ClusterIDConfigMapKey = "cluster-id"
)
