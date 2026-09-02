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

package forge

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

func TestVirtualKubeletDeploymentHostNetworkOptions(t *testing.T) {
	nodeSelector := map[string]string{"kubernetes.io/hostname": "node-1"}
	deployment := VirtualKubeletDeployment("home", "liqo", []string{"10.0.0.0/16"}, &offloadingv1beta1.VirtualNode{
		Spec: offloadingv1beta1.VirtualNodeSpec{
			ClusterID: liqov1beta1.ClusterID("remote"),
		},
	}, &offloadingv1beta1.VkOptionsTemplate{
		Spec: offloadingv1beta1.VkOptionsTemplateSpec{
			HostNetwork:  true,
			NodeSelector: nodeSelector,
		},
	})

	podSpec := deployment.Spec.Template.Spec
	if !podSpec.HostNetwork {
		t.Fatal("expected hostNetwork to be enabled")
	}
	if podSpec.DNSPolicy != corev1.DNSClusterFirstWithHostNet {
		t.Fatalf("expected DNS policy %q, got %q", corev1.DNSClusterFirstWithHostNet, podSpec.DNSPolicy)
	}
	if !reflect.DeepEqual(podSpec.NodeSelector, nodeSelector) {
		t.Fatalf("expected node selector %v, got %v", nodeSelector, podSpec.NodeSelector)
	}
}
