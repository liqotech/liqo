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

package virtualnode

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

func testVirtualNode(name string) *offloadingv1beta1.VirtualNode {
	return &offloadingv1beta1.VirtualNode{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "tenant-ns"},
	}
}

func admissionRequest(t *testing.T, vn *offloadingv1beta1.VirtualNode, op admissionv1.Operation) admission.Request {
	t.Helper()
	raw, err := json.Marshal(vn)
	if err != nil {
		t.Fatalf("failed marshaling the virtualnode: %v", err)
	}
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: op,
		Object:    runtime.RawExtension{Raw: raw},
	}}
}

func testClientBuilder(t *testing.T, objects ...runtime.Object) *fake.ClientBuilder {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed registering the corev1 scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...)
}

func TestValidator(t *testing.T) {
	const vnName = "virtual-node-1"

	t.Run("creation is denied when a node with the same name exists", func(t *testing.T) {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: vnName}}
		cl := testClientBuilder(t, node).Build()

		res := NewValidator(cl).Handle(context.TODO(), admissionRequest(t, testVirtualNode(vnName), admissionv1.Create))
		if res.Allowed {
			t.Error("expected the request to be denied")
		}
		if !strings.Contains(res.Result.Message, "node virtual-node-1 already exists") {
			t.Errorf("unexpected denial message: %q", res.Result.Message)
		}
	})

	t.Run("creation is allowed when no node with the same name exists", func(t *testing.T) {
		cl := testClientBuilder(t).Build()

		res := NewValidator(cl).Handle(context.TODO(), admissionRequest(t, testVirtualNode(vnName), admissionv1.Create))
		if !res.Allowed {
			t.Errorf("expected the request to be allowed, got: %v", res.Result)
		}
	})

	t.Run("update is allowed even when a node with the same name exists", func(t *testing.T) {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: vnName}}
		cl := testClientBuilder(t, node).Build()

		res := NewValidator(cl).Handle(context.TODO(), admissionRequest(t, testVirtualNode(vnName), admissionv1.Update))
		if !res.Allowed {
			t.Errorf("expected the request to be allowed, got: %v", res.Result)
		}
	})
}
