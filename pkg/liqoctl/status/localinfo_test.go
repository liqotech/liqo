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

package status

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

var _ = Describe("LocalInfo", func() {
	const (
		clusterID    = "fake"
		clusterName  = "fake"
		namespace    = "liqo"
		rootTitle    = "Local Cluster Informations"
		serviceCIDR  = "10.80.0.0/12"
		externalCIDR = "10.201.0.0/16"
		podCIDR      = "10.200.0.0/16"
	)

	var (
		reservedSubnets = []string{"10.202.0.0/16"}
		clientBuilder   fake.ClientBuilder
		rootNode        = newRootInfoNode(rootTitle)
		lic             *LocalInfoChecker
		ctx             = context.Background()
		clusterIdentity *v1alpha1.ClusterIdentity
		networkConfig   *getters.NetworkConfig
	)

	BeforeEach(func() {
		ctx = context.Background()
		_ = netv1.AddToScheme(scheme.Scheme)
		clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)
	})

	Context("Creating a new LocalInfoChecker", func() {
		JustBeforeEach(func() {
			lic = newLocalInfoChecker(namespace, clientBuilder.Build())
		})
		It("should return a valid LocalInfoChecker", func() {
			licTest := &LocalInfoChecker{
				client:            clientBuilder.Build(),
				namespace:         namespace,
				errors:            false,
				collectionErrors:  nil,
				rootLocalInfoNode: rootNode,
			}
			Expect(*lic).To(Equal(*licTest))
		})
	})
	Context("Getting a local cluster identity", func() {
		BeforeEach(func() {
			clientBuilder.WithObjects(&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind: "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": "clusterid-configmap",
					},
					Namespace: namespace,
				},
				Immutable: new(bool),
				Data: map[string]string{
					"CLUSTER_ID":   clusterID,
					"CLUSTER_NAME": clusterName,
				},
				BinaryData: map[string][]byte{},
			})
		})
		JustBeforeEach(func() {
			clientBuilder.Build()
			clusterIdentity, _ = getLocalClusterIdentity(ctx, clientBuilder.Build(), namespace)
		})
		It("should return a valid cluster identity", func() {
			Expect(clusterIdentity.ClusterID).To(Equal(clusterID))
			Expect(clusterIdentity.ClusterName).To(Equal(clusterName))
		})
	})
	Context("Getting a network config", func() {
		BeforeEach(func() {
			clientBuilder.WithObjects(&netv1.IpamStorage{
				TypeMeta: metav1.TypeMeta{
					Kind: "IpamStorage",
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"net.liqo.io/ipamstorage": "true",
					},
					Namespace: "default",
				},
				Spec: netv1.IpamSpec{
					ReservedSubnets: reservedSubnets,
					ExternalCIDR:    externalCIDR,
					PodCIDR:         podCIDR,
					ServiceCIDR:     serviceCIDR,
				},
			})
		})
		JustBeforeEach(func() {
			networkConfig, _ = getLocalNetworkConfig(ctx, clientBuilder.Build())
		})
		It("should return a valid network config", func() {
			Expect(networkConfig.ExternalCIDR).To(Equal(externalCIDR))
			Expect(networkConfig.PodCIDR).To(Equal(podCIDR))
			Expect(networkConfig.ReservedSubnets).To(Equal(reservedSubnets))
			Expect(networkConfig.ServiceCIDR).To(Equal(serviceCIDR))
		})
	})
})
