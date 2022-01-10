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

package add

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

const (
	localClusterID    string = "local-cluster-id"
	remoteTokenPrefix string = "remote-token-"
)

var (
	ctx         context.Context
	clientset   *fake.Clientset
	k8sClient   client.Client
	t, r, check *ClusterArgs
	err         error
)

func TestAddCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Parameters Fetching")
}

var _ = BeforeSuite(func() {
	utilruntime.Must(discoveryv1alpha1.AddToScheme(scheme.Scheme))
})

var _ = Describe("Extract elements from apiServer", func() {
	AssertValidAddition := func() {
		By("Auth token was correctly created")
		secret, err := clientset.CoreV1().Secrets(check.Namespace).Get(ctx, remoteTokenPrefix+check.ClusterID, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(secret.StringData["token"]).To(BeEquivalentTo(check.ClusterToken))
		// ForeignCluster exists and has the required values
		fc, err := foreigncluster.GetForeignClusterByID(ctx, k8sClient, check.ClusterID)
		Expect(err).ToNot(HaveOccurred())
		Expect(fc.Spec.ForeignAuthURL).To(BeEquivalentTo(check.ClusterAuthURL))
		Expect(fc.Spec.OutgoingPeeringEnabled).To(BeEquivalentTo(discoveryv1alpha1.PeeringEnabledYes))
		Expect(fc.Spec.IncomingPeeringEnabled).To(BeEquivalentTo(discoveryv1alpha1.PeeringEnabledAuto))
		Expect(fc.Spec.InsecureSkipTLSVerify).To(BeEquivalentTo(pointer.BoolPtr(true)))
	}
	BeforeEach(func() {
		clientset, k8sClient = setUpEnvironment(&ClusterArgs{
			ClusterName:    "cluster-local",
			ClusterToken:   "test-token",
			ClusterAuthURL: "https://example.com",
			ClusterID:      "test-cluster-id",
			Namespace:      "liqo",
		})
	})

	When("Add a cluster with a ClusterID equal to the local one", func() {
		BeforeEach(func() {
			t = &ClusterArgs{
				ClusterName:    "cluster-local",
				ClusterToken:   "test-token",
				ClusterAuthURL: "https://example.com",
				ClusterID:      localClusterID,
				Namespace:      "liqo",
			}
		})
		JustBeforeEach(func() {
			err = processAddCluster(ctx, t, clientset, k8sClient)
		})
		It("Add should fail", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(BeEquivalentTo(sameClusterError))
		})
	})
	When("Add a cluster with a ClusterID", func() {
		BeforeEach(func() {
			t = &ClusterArgs{
				ClusterName:    "cluster1bis",
				ClusterToken:   "test-token",
				ClusterAuthURL: "https://example.com",
				ClusterID:      "test-cluster-id2",
				Namespace:      "liqo",
			}
			check = t
		})
		JustBeforeEach(func() {
			err = processAddCluster(ctx, t, clientset, k8sClient)
		})
		It("Should create the following resources", AssertValidAddition)
		When("Add a cluster with the same ClusterID", func() {
			BeforeEach(func() {
				r = &ClusterArgs{
					ClusterName:    "cluster1bis",
					ClusterToken:   "test-token2",
					ClusterAuthURL: "https://test.com",
					ClusterID:      "test-cluster-id2",
					Namespace:      "liqo",
				}
				check = r
			})
			JustBeforeEach(func() {
				err = processAddCluster(ctx, r, clientset, k8sClient)
			})
			It("Should enforce the new values", AssertValidAddition)
		})
	})

})

func setUpEnvironment(t *ClusterArgs) (*fake.Clientset, client.Client) {
	// Create Namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: t.Namespace,
		},
	}
	// Create ClusterID ConfigMap
	clusterIDConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "clusterid-configmap",
			Namespace: t.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component": "clusterid-configmap",
				"app.kubernetes.io/name":      "clusterid-configmap",
			},
		},
		Data: map[string]string{
			consts.ClusterIDConfigMapKey: localClusterID,
		},
	}
	clientSet := fake.NewSimpleClientset(ns, clusterIDConfigMap)
	k8sClient := ctrlfake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(ns, clusterIDConfigMap).Build()
	return clientSet, k8sClient
}
