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

package tenant_test

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	testutil "github.com/liqotech/liqo/pkg/utils/testutil"
)

var scheme *runtime.Scheme

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	testutil.LogsToGinkgoWriter()
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(authv1beta1.AddToScheme(scheme)).To(Succeed())
})

func TestTenantWebhooks(t *testing.T) {
	RegisterFailHandler(Fail)
	klog.InitFlags(nil)
	RunSpecs(t, "Tenant Suite")
}

func generateFakeTenant(name, namespace, clusterID string) *authv1beta1.Tenant {
	return &authv1beta1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				consts.RemoteClusterID: clusterID,
			},
			UID: uuid.NewUUID(),
		},
		Spec: authv1beta1.TenantSpec{
			ClusterID: liqov1beta1.ClusterID(clusterID),
		},
	}
}

func generateAdmissionRequest(tenant *authv1beta1.Tenant, op admissionv1.Operation) admission.Request {
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object:    tenantToRawExtension(tenant),
			Operation: op,
		},
	}
}

func tenantToRawExtension(tenant *authv1beta1.Tenant) runtime.RawExtension {
	marshaledTenant, err := json.Marshal(tenant)
	Expect(err).To(BeNil())
	return runtime.RawExtension{Raw: marshaledTenant}
}
