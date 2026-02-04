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

package foreigncluster_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/foreigncluster"
)

var _ = Describe("GetForeignClusterByID function", func() {
	const (
		testClusterID1 = liqov1beta1.ClusterID("test-cluster-1")
		testClusterID2 = liqov1beta1.ClusterID("test-cluster-2")
		testClusterID3 = liqov1beta1.ClusterID("test-cluster-3")
		testClusterID4 = liqov1beta1.ClusterID("test-cluster-4")
		nonExistentID  = liqov1beta1.ClusterID("non-existent-cluster")
	)

	var (
		ctx    context.Context
		scheme *runtime.Scheme
		cl     client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(liqov1beta1.AddToScheme(scheme)).To(Succeed())
	})

	Context("Fallback #1: Label-based lookup (fast path)", func() {
		BeforeEach(func() {
			// Create a ForeignCluster with the standard label
			fc := &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fc-with-label",
					Labels: map[string]string{
						consts.RemoteClusterID: string(testClusterID1),
					},
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: testClusterID1,
				},
			}
			cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(fc).Build()
		})

		It("should find ForeignCluster by label", func() {
			fc, err := foreigncluster.GetForeignClusterByID(ctx, cl, testClusterID1)
			Expect(err).ToNot(HaveOccurred())
			Expect(fc).ToNot(BeNil())
			Expect(fc.Spec.ClusterID).To(Equal(testClusterID1))
			Expect(fc.Name).To(Equal("fc-with-label"))
		})

		It("should return not found for non-existent cluster", func() {
			fc, err := foreigncluster.GetForeignClusterByID(ctx, cl, nonExistentID)
			Expect(err).To(HaveOccurred())
			Expect(kerrors.IsNotFound(err)).To(BeTrue())
			Expect(fc).To(BeNil())
		})
	})

	Context("Fallback #2: Name-based lookup (out-of-band peering)", func() {
		BeforeEach(func() {
			// Create a ForeignCluster without the label, but with name == clusterID
			// This simulates manual/out-of-band creation
			fc := &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: string(testClusterID2), // Name matches clusterID
					// Note: No liqo.io/remote-cluster-id label
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: testClusterID2,
				},
			}
			cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(fc).Build()
		})

		It("should find ForeignCluster by name when label is missing", func() {
			fc, err := foreigncluster.GetForeignClusterByID(ctx, cl, testClusterID2)
			Expect(err).ToNot(HaveOccurred())
			Expect(fc).ToNot(BeNil())
			Expect(fc.Spec.ClusterID).To(Equal(testClusterID2))
			Expect(fc.Name).To(Equal(string(testClusterID2)))
		})

		It("should validate spec.ClusterID matches requested ID", func() {
			// Create a ForeignCluster where name matches but spec.ClusterID doesn't
			fcMismatch := &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: string(testClusterID3),
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: "different-cluster-id", // Mismatch!
				},
			}
			cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(fcMismatch).Build()

			fc, err := foreigncluster.GetForeignClusterByID(ctx, cl, testClusterID3)
			Expect(err).To(HaveOccurred())
			Expect(kerrors.IsNotFound(err)).To(BeTrue())
			Expect(fc).To(BeNil())
		})
	})

	Context("Fallback #3: Exhaustive search (expensive operation)", func() {
		BeforeEach(func() {
			// Create multiple ForeignClusters, none with the correct label or name
			// Only one has the matching spec.ClusterID
			fc1 := &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "random-name-1",
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: "some-other-cluster",
				},
			}
			fc2 := &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "random-name-2",
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: testClusterID4, // This is the one we're looking for
				},
			}
			fc3 := &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "random-name-3",
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: "yet-another-cluster",
				},
			}
			cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(fc1, fc2, fc3).Build()
		})

		It("should find ForeignCluster via exhaustive search when label and name lookups fail", func() {
			fc, err := foreigncluster.GetForeignClusterByID(ctx, cl, testClusterID4)
			Expect(err).ToNot(HaveOccurred())
			Expect(fc).ToNot(BeNil())
			Expect(fc.Spec.ClusterID).To(Equal(testClusterID4))
			Expect(fc.Name).To(Equal("random-name-2"))
		})

		It("should return not found when exhaustive search finds nothing", func() {
			fc, err := foreigncluster.GetForeignClusterByID(ctx, cl, nonExistentID)
			Expect(err).To(HaveOccurred())
			Expect(kerrors.IsNotFound(err)).To(BeTrue())
			Expect(fc).To(BeNil())
		})
	})

	Context("Multiple ForeignClusters with same label", func() {
		var (
			fc1 *liqov1beta1.ForeignCluster
			fc2 *liqov1beta1.ForeignCluster
		)

		BeforeEach(func() {
			// Create two ForeignClusters with the same label
			// The function should return the older one based on creationTimestamp
			fc1 = &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fc-older",
					Labels: map[string]string{
						consts.RemoteClusterID: string(testClusterID1),
					},
					CreationTimestamp: metav1.Time{Time: metav1.Now().Add(-24 * time.Hour)}, // 1 day ago
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: testClusterID1,
				},
			}
			fc2 = &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fc-newer",
					Labels: map[string]string{
						consts.RemoteClusterID: string(testClusterID1),
					},
					CreationTimestamp: metav1.Now(), // Now
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: testClusterID1,
				},
			}
			cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(fc1, fc2).Build()
		})

		It("should return the older ForeignCluster when multiple exist", func() {
			fc, err := foreigncluster.GetForeignClusterByID(ctx, cl, testClusterID1)
			Expect(err).ToNot(HaveOccurred())
			Expect(fc).ToNot(BeNil())
			Expect(fc.Name).To(Equal("fc-older"))
		})
	})

	Context("Edge cases", func() {
		It("should handle empty clusterID gracefully", func() {
			cl = fake.NewClientBuilder().WithScheme(scheme).Build()
			fc, err := foreigncluster.GetForeignClusterByID(ctx, cl, "")
			Expect(err).To(HaveOccurred())
			Expect(fc).To(BeNil())
		})

		It("should handle ForeignCluster with empty spec.ClusterID in name-based lookup", func() {
			// ForeignCluster with name matching but empty spec.ClusterID should still be found
			fcEmpty := &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: string(testClusterID2),
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: "", // Empty
				},
			}
			cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(fcEmpty).Build()

			fc, err := foreigncluster.GetForeignClusterByID(ctx, cl, testClusterID2)
			Expect(err).ToNot(HaveOccurred())
			Expect(fc).ToNot(BeNil())
			Expect(fc.Name).To(Equal(string(testClusterID2)))
		})
	})

	Context("Combination scenarios (testing fallback chain)", func() {
		It("should prefer label-based lookup over name-based when both exist", func() {
			// Create two ForeignClusters: one with label, one matching by name
			fcWithLabel := &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fc-with-label",
					Labels: map[string]string{
						consts.RemoteClusterID: string(testClusterID1),
					},
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: testClusterID1,
				},
			}
			fcByName := &liqov1beta1.ForeignCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: string(testClusterID1), // Name matches
				},
				Spec: liqov1beta1.ForeignClusterSpec{
					ClusterID: testClusterID1,
				},
			}
			cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(fcWithLabel, fcByName).Build()

			fc, err := foreigncluster.GetForeignClusterByID(ctx, cl, testClusterID1)
			Expect(err).ToNot(HaveOccurred())
			Expect(fc).ToNot(BeNil())
			// Should return the one found by label (fast path)
			Expect(fc.Name).To(Equal("fc-with-label"))
		})
	})
})
