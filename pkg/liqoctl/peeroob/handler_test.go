// Copyright 2019-2023 The Liqo Authors
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

package peeroob

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/peer"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

const (
	localClusterID    string = "local-cluster-id"
	localClusterName  string = "local-cluster-name"
	remoteTokenPrefix string = "remote-token-"
)

var _ = Describe("Test Peer Command", func() {
	var (
		ctx     context.Context
		options *Options

		err error
	)

	BeforeEach(func() {
		ctx = context.Background()
		options = &Options{
			Options: &peer.Options{
				Factory:     &factory.Factory{LiqoNamespace: "liqo-non-standard"},
				ClusterName: "remote-cluster-name",
			},
			ClusterID:      "remote-cluster-id",
			ClusterToken:   "remote-token",
			ClusterAuthURL: "https://remote.auth",
		}

		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: options.LiqoNamespace}}
		clusterIDConfigMap := testutil.FakeClusterIDConfigMap(options.LiqoNamespace, localClusterID, localClusterName)

		options.Factory.KubeClient = fake.NewSimpleClientset(ns, clusterIDConfigMap)
		options.Factory.CRClient = ctrlfake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(ns, clusterIDConfigMap).Build()
	})

	JustBeforeEach(func() { _, err = options.peer(ctx) })

	When("adding a cluster with a ClusterID equal to the local one", func() {
		BeforeEach(func() { options.ClusterID = localClusterID })
		It("should fail", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("adding a cluster with a valid ClusterID", func() {
		ItBody := func() {
			By("checking existence and correctness of the authentication token")
			secret, err := options.KubeClient.CoreV1().Secrets(options.LiqoNamespace).Get(ctx, remoteTokenPrefix+options.ClusterID, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(secret.StringData["token"]).To(BeEquivalentTo(options.ClusterToken))

			By("checking existence and correctness of the ForeignCluster")
			fc, err := foreigncluster.GetForeignClusterByID(ctx, options.CRClient, options.ClusterID)
			Expect(err).ToNot(HaveOccurred())
			Expect(fc.Spec.ForeignAuthURL).To(BeEquivalentTo(options.ClusterAuthURL))
			Expect(fc.Spec.OutgoingPeeringEnabled).To(BeEquivalentTo(discoveryv1alpha1.PeeringEnabledYes))
			Expect(fc.Spec.IncomingPeeringEnabled).To(BeEquivalentTo(discoveryv1alpha1.PeeringEnabledAuto))
			Expect(fc.Spec.InsecureSkipTLSVerify).To(BeEquivalentTo(pointer.BoolPtr(true)))
		}

		It("should correctly create the appropriate resources", ItBody)

		When("adding a cluster with the same ClusterID", func() {
			BeforeEach(func() {
				options.ClusterName = "remote-cluster-name-new"
				options.ClusterAuthURL = "https://remote.auth.new"
				options.ClusterToken = "remote-token-new"
			})

			JustBeforeEach(func() { _, err = options.peer(ctx) })
			It("should enforce the new values", ItBody)
		})
	})

})
