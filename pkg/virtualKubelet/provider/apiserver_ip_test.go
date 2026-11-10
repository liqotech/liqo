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

package provider

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	liqofake "github.com/liqotech/liqo/pkg/client/clientset/versioned/fake"
	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("Kubernetes service IP mapper", func() {
	const liqoNamespace = "liqo"

	kubernetesService := func(clusterIPs ...string) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: metav1.NamespaceDefault},
			Spec:       corev1.ServiceSpec{ClusterIP: clusterIPs[0], ClusterIPs: clusterIPs},
		}
	}

	apiServerIP := func(remapped string) *ipamv1alpha1.IP {
		return &ipamv1alpha1.IP{
			ObjectMeta: metav1.ObjectMeta{
				Name: "api-server", Namespace: liqoNamespace,
				Labels: map[string]string{consts.IPTypeLabelKey: consts.IPTypeAPIServer},
			},
			Status: ipamv1alpha1.IPStatus{IP: networkingv1beta1.IP(remapped)},
		}
	}

	DescribeTable("mapper", func(svc *corev1.Service, ips []*ipamv1alpha1.IP, expected []string, expectErr bool) {
		var liqoObjs []runtime.Object
		for _, ip := range ips {
			liqoObjs = append(liqoObjs, ip)
		}
		mapper := newKubernetesServiceIPMapper(fake.NewSimpleClientset(svc), liqofake.NewSimpleClientset(liqoObjs...), liqoNamespace)

		res, err := mapper(context.Background())
		if expectErr {
			Expect(err).To(HaveOccurred())
			return
		}
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(expected))
	},
		Entry("remapped IP available", kubernetesService("10.96.0.1"), []*ipamv1alpha1.IP{apiServerIP("10.70.0.1")}, []string{"10.70.0.1"}, false),
		Entry("IPv4 ClusterIP, mapping not yet created (must wait)", kubernetesService("10.96.0.1"), nil, nil, true),
		Entry("dual-stack IPv6-primary, mapping not yet created (must wait)",
			kubernetesService("fd00:10:96::1", "10.96.0.1"), nil, nil, true),
		Entry("IPv6 single-stack service: remapping skipped, no addresses and no error",
			kubernetesService("fd00:10:96::1"), nil, nil, false),
	)
})
