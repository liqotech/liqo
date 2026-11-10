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

package gwmasqbypass

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

func TestPodFwUsesGivenIPv4(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: "ns"},
		Status:     corev1.PodStatus{PodIP: "fd00::5", PodIPs: []corev1.PodIP{{IP: "fd00::5"}, {IP: "10.1.2.3"}}},
	}
	var rules []firewall.NatRule
	addPodToFw(pod, "10.1.2.3", &rules, 6091)
	if len(rules) != 1 || *rules[0].To != "10.1.2.3" || rules[0].Match[0].IP.Value != "10.1.2.3" {
		t.Fatalf("unexpected rule on add: %+v", rules)
	}
	updatePodToFw(pod, "10.1.2.4", &rules[0], 6091)
	if *rules[0].To != "10.1.2.4" || rules[0].Match[0].IP.Value != "10.1.2.4" {
		t.Fatalf("unexpected rule on update: %+v", rules[0])
	}
}
