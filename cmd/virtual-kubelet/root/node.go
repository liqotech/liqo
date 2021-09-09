// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package root

import (
	"context"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/liqotech/liqo/cmd/virtual-kubelet/provider"
	"github.com/liqotech/liqo/internal/utils/errdefs"
)

const (
	osLabel     = "beta.kubernetes.io/os"
	taintKey    = "VKUBELET_TAINT_KEY"
	taintValue  = "VKUBELET_TAINT_VALUE"
	taintEffect = "VKUBELET_TAINT_EFFECT"
)

// NodeFromProvider builds a kubernetes node object from a provider
// This is a temporary solution until node stuff actually split off from the provider interface itself.
func NodeFromProvider(ctx context.Context, name string, p provider.Provider, version string, refs []metav1.OwnerReference,
	nodeExtraAnnotations, nodeExtraLabels map[string]string) (*corev1.Node, error) {
	taints := make([]corev1.Taint, 0)

	taint, err := buildTaint()
	if err != nil {
		return nil, err
	}

	if taint != nil {
		taints = append(taints, *taint)
	}

	annotations := nodeExtraAnnotations
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"type":                   "virtual-kubelet",
				"kubernetes.io/role":     "agent",
				"kubernetes.io/hostname": name,
			},
			Annotations: annotations,
		},
		Spec: corev1.NodeSpec{
			Taints: taints,
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				Architecture:   "amd64",
				KubeletVersion: version,
			},
		},
	}
	if len(refs) > 0 {
		node.SetOwnerReferences(refs)
	}

	for k, v := range nodeExtraLabels {
		node.ObjectMeta.Labels[k] = v
	}

	p.ConfigureNode(ctx, node)
	if _, ok := node.ObjectMeta.Labels[osLabel]; !ok {
		node.ObjectMeta.Labels[osLabel] = strings.ToLower(node.Status.NodeInfo.OperatingSystem)
	}

	return node, nil
}

// buildTaint creates a taint using the provided key/value.
// Taint effect is read from the environment
// The taint key/value may be overwritten by the environment.
func buildTaint() (*corev1.Taint, error) {
	keyEnv, ok1 := os.LookupEnv(taintKey)
	valueEnv, ok2 := os.LookupEnv(taintValue)
	effectEnv, ok3 := os.LookupEnv(taintEffect)

	if !ok1 || !ok2 || !ok3 {
		return nil, nil
	}

	var effect corev1.TaintEffect
	switch effectEnv {
	case "NoSchedule":
		effect = corev1.TaintEffectNoSchedule
	case "NoExecute":
		effect = corev1.TaintEffectNoExecute
	case "PreferNoSchedule":
		effect = corev1.TaintEffectPreferNoSchedule
	default:
		return nil, errdefs.InvalidInputf("taint effect %q is not supported", effectEnv)
	}

	return &corev1.Taint{
		Key:    keyEnv,
		Value:  valueEnv,
		Effect: effect,
	}, nil
}
