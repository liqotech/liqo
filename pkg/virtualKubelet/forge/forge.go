// Copyright 2019-2024 The Liqo Authors
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
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
)

// ReflectionFieldManager -> The name associated with the fields modified by virtual kubelet reflection.
const ReflectionFieldManager = "reflection.liqo.io"

var (
	// LocalCluster -> the cluster id associated with the local cluster.
	LocalCluster discoveryv1alpha1.ClusterID
	// RemoteCluster -> the cluster id associated with the remote cluster.
	RemoteCluster discoveryv1alpha1.ClusterID

	// LiqoNodeName -> the name of the node associated with the current virtual-kubelet.
	LiqoNodeName string
	// LiqoNodeIP -> the local IP of the node associated with the current virtual-kubelet.
	LiqoNodeIP string
	// StartTime -> the instant in time the forging logic has been started.
	StartTime time.Time

	// KubernetesServicePort -> the port of the kubernetes.default service.
	KubernetesServicePort string
)

// Init initializes the forging logic.
func Init(localCluster, remoteCluster discoveryv1alpha1.ClusterID, nodeName, nodeIP string) {
	LocalCluster = localCluster
	RemoteCluster = remoteCluster

	LiqoNodeName = nodeName
	LiqoNodeIP = nodeIP
	StartTime = time.Now().Truncate(time.Second)

	// The kubernetes service port is directly retrieved from the corresponding environment variable,
	// since it is the one used locally. In case it is not found, it is defaulted to 443.
	KubernetesServicePort = os.Getenv("KUBERNETES_SERVICE_PORT")
	if KubernetesServicePort == "" {
		KubernetesServicePort = "443"
	}
}

// ApplyOptions returns the apply options configured for object reflection.
func ApplyOptions() metav1.ApplyOptions {
	return metav1.ApplyOptions{
		Force:        true,
		FieldManager: ReflectionFieldManager,
	}
}

// ForgingOpts contains options to forge the reflected resources.
type ForgingOpts struct {
	LabelsNotReflected      []string
	AnnotationsNotReflected []string
	NodeSelector            map[string]string
	Tolerations             []corev1.Toleration
	Affinity                *virtualkubeletv1alpha1.Affinity
}

// NewForgingOpts returns a new ForgingOpts instance.
func NewForgingOpts(offloadingPatch *virtualkubeletv1alpha1.OffloadingPatch) ForgingOpts {
	if offloadingPatch == nil {
		return NewEmptyForgingOpts()
	}

	return ForgingOpts{
		LabelsNotReflected:      offloadingPatch.LabelsNotReflected,
		AnnotationsNotReflected: offloadingPatch.AnnotationsNotReflected,
		NodeSelector:            offloadingPatch.NodeSelector,
		Tolerations:             offloadingPatch.Tolerations,
		Affinity:                offloadingPatch.Affinity,
	}
}

// NewEmptyForgingOpts returns a new ForgingOpts instance with empty fields.
func NewEmptyForgingOpts() ForgingOpts {
	return ForgingOpts{}
}
