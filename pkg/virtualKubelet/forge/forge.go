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

package forge

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReflectionFieldManager -> The name associated with the fields modified by virtual kubelet reflection.
const ReflectionFieldManager = "reflection.liqo.io"

var (
	// LocalClusterID -> the cluster ID associated with the local cluster.
	LocalClusterID string
	// RemoteClusterID -> the cluster ID associated with the remote cluster.
	RemoteClusterID string
	// LiqoNodeName -> the name of the node associated with the current virtual-kubelet.
	LiqoNodeName string
	// LiqoNodeIP -> the local IP of the node associated with the current virtual-kubelet.
	LiqoNodeIP string
	// StartTime -> the instant in time the forging logic has been started.
	StartTime time.Time
)

// Init initializes the forging logic.
func Init(localClusterID, remoteClusterID, nodeName, nodeIP string) {
	LocalClusterID = localClusterID
	RemoteClusterID = remoteClusterID
	LiqoNodeName = nodeName
	LiqoNodeIP = nodeIP
	StartTime = time.Now().Truncate(time.Second)
}

// ApplyOptions returns the apply options configured for object reflection.
func ApplyOptions() metav1.ApplyOptions {
	return metav1.ApplyOptions{
		Force:        true,
		FieldManager: ReflectionFieldManager,
	}
}
