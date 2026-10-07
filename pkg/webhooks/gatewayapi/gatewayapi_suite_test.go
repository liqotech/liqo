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

package gatewayapi

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	testutil "github.com/liqotech/liqo/pkg/utils/testutil"
)

func TestGatewayAPIWebhooks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gateway API Webhooks Suite")
}

var _ = BeforeSuite(func() { testutil.LogsToGinkgoWriter() })

// request returns an admission request for the given object (and old object, in case of updates).
func request(kind string, op admissionv1.Operation, obj, old metav1.Object) admission.Request {
	raw := func(o metav1.Object) runtime.RawExtension {
		if o == nil {
			return runtime.RawExtension{}
		}
		marshaled, err := json.Marshal(o)
		Expect(err).ToNot(HaveOccurred())
		return runtime.RawExtension{Raw: marshaled}
	}

	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Kind:      metav1.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: kind},
		Namespace: obj.GetNamespace(), Name: obj.GetName(),
		Operation: op, Object: raw(obj), OldObject: raw(old),
	}}
}
