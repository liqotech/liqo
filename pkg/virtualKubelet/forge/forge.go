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

package forge

import (
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

// ReflectionFieldManager -> The name associated with the fields modified by virtual kubelet reflection.
const ReflectionFieldManager = "reflection.liqo.io"

var (
	// LocalCluster -> the cluster id associated with the local cluster.
	LocalCluster liqov1beta1.ClusterID
	// RemoteCluster -> the cluster id associated with the remote cluster.
	RemoteCluster liqov1beta1.ClusterID

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
func Init(localCluster, remoteCluster liqov1beta1.ClusterID, nodeName, nodeIP string) {
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
	Affinity                *offloadingv1beta1.Affinity
	RuntimeClassName        *string
}

// NewForgingOpts returns a new ForgingOpts instance.
func NewForgingOpts(offloadingPatch *offloadingv1beta1.OffloadingPatch) ForgingOpts {
	if offloadingPatch == nil {
		return NewEmptyForgingOpts()
	}

	return ForgingOpts{
		LabelsNotReflected:      offloadingPatch.LabelsNotReflected,
		AnnotationsNotReflected: offloadingPatch.AnnotationsNotReflected,
		NodeSelector:            offloadingPatch.NodeSelector,
		Tolerations:             offloadingPatch.Tolerations,
		Affinity:                offloadingPatch.Affinity,
		RuntimeClassName:        offloadingPatch.RuntimeClassName,
	}
}

// NewEmptyForgingOpts returns a new ForgingOpts instance with empty fields.
func NewEmptyForgingOpts() ForgingOpts {
	return ForgingOpts{}
}
