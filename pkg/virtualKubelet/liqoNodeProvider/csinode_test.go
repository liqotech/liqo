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

package liqonodeprovider

import (
	"context"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("CSINode", func() {

	var (
		cluster      testutil.Cluster
		client       kubernetes.Interface
		nodeProvider *LiqoNodeProvider
		ctx          context.Context
		cancel       context.CancelFunc
		err          error
	)

	// createNode creates the node managed by the provider, returning the persisted object (i.e. with its UID).
	createNode := func() *corev1.Node {
		node, err := client.CoreV1().Nodes().Create(ctx, nodeProvider.GetNode(), metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
		return node
	}

	// getCSINode returns the CSINode object associated with the node, or nil if it does not exist.
	getCSINode := func() *storagev1.CSINode {
		csiNode, err := client.StorageV1().CSINodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return nil
		}
		return csiNode
	}

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		cluster, _, err = testutil.NewTestCluster([]string{
			filepath.Join("..", "..", "..", "deployments", "liqo", "charts", "liqo-crds", "crds"),
		})
		Expect(err).To(BeNil())

		client = cluster.GetClient()

		nodeProvider = NewLiqoNodeProvider(&InitConfig{
			NodeName:  nodeName,
			Namespace: kubeletNamespace,

			HomeConfig:      cluster.GetCfg(),
			RemoteConfig:    cluster.GetCfg(), /* not actually used in tests */
			RemoteClusterID: foreignClusterID,
		})
	})

	AfterEach(func() {
		cancel()

		err := cluster.GetEnv().Stop()
		Expect(err).To(BeNil())
	})

	It("Should create a CSINode with no drivers, controlled by the node", func() {
		node := createNode()

		Expect(nodeProvider.EnsureCSINode(ctx)).To(Succeed())

		csiNode := getCSINode()
		Expect(csiNode).ToNot(BeNil())
		Expect(csiNode.GetName()).To(Equal(node.GetName()))
		Expect(csiNode.Spec.Drivers).To(BeEmpty())

		Expect(csiNode.OwnerReferences).To(HaveLen(1))
		ownerRef := csiNode.OwnerReferences[0]
		Expect(ownerRef.APIVersion).To(Equal("v1"))
		Expect(ownerRef.Kind).To(Equal("Node"))
		Expect(ownerRef.Name).To(Equal(node.GetName()))
		Expect(ownerRef.UID).To(Equal(node.GetUID()))
		Expect(ownerRef.Controller).ToNot(BeNil())
		Expect(*ownerRef.Controller).To(BeTrue())
	})

	It("Should not modify an existing CSINode controlled by the node", func() {
		node := createNode()
		Expect(nodeProvider.EnsureCSINode(ctx)).To(Succeed())

		// Simulate a CSI driver registered on the node.
		csiNode := getCSINode()
		csiNode.Spec.Drivers = []storagev1.CSINodeDriver{{Name: "driver.csi.liqo.io", NodeID: node.GetName()}}
		csiNode, err = client.StorageV1().CSINodes().Update(ctx, csiNode, metav1.UpdateOptions{})
		Expect(err).ToNot(HaveOccurred())
		resourceVersion := csiNode.GetResourceVersion()

		Expect(nodeProvider.EnsureCSINode(ctx)).To(Succeed())

		csiNode = getCSINode()
		Expect(csiNode.GetResourceVersion()).To(Equal(resourceVersion))
		Expect(csiNode.Spec.Drivers).To(HaveLen(1))
		Expect(csiNode.Spec.Drivers[0].Name).To(Equal("driver.csi.liqo.io"))
	})

	It("Should tolerate an existing CSINode without an owner reference", func() {
		node := createNode()

		// Emulate a CSINode object created manually, hence without an owner reference.
		manualCSINode, err := client.StorageV1().CSINodes().Create(ctx, &storagev1.CSINode{
			ObjectMeta: metav1.ObjectMeta{Name: node.GetName()},
			Spec:       storagev1.CSINodeSpec{Drivers: []storagev1.CSINodeDriver{}},
		}, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		Expect(nodeProvider.EnsureCSINode(ctx)).To(Succeed())

		csiNode := getCSINode()
		Expect(csiNode.GetResourceVersion()).To(Equal(manualCSINode.GetResourceVersion()))
		Expect(csiNode.GetOwnerReferences()).To(BeEmpty())
	})

	It("Should fail if the node does not exist", func() {
		Expect(nodeProvider.ensureCSINode(ctx)).ToNot(Succeed())
		Expect(getCSINode()).To(BeNil())
	})

	It("Should fail if the CSINode is owned by a previous incarnation of the node", func() {
		createNode()
		Expect(nodeProvider.EnsureCSINode(ctx)).To(Succeed())
		previousCSINode := getCSINode()

		// Re-create the node, which gets a new UID.
		Expect(client.CoreV1().Nodes().Delete(ctx, nodeName, metav1.DeleteOptions{})).To(Succeed())
		node := createNode()
		Expect(node.GetUID()).ToNot(Equal(previousCSINode.OwnerReferences[0].UID))

		// The stale CSINode has not been garbage collected yet: the creation is expected to fail and no
		// modification is performed on the existing object.
		Expect(nodeProvider.ensureCSINode(ctx)).ToNot(Succeed())
		Expect(getCSINode().GetResourceVersion()).To(Equal(previousCSINode.GetResourceVersion()))
	})

	It("Should create the CSINode once the stale one has been garbage collected", func() {
		createNode()
		Expect(nodeProvider.EnsureCSINode(ctx)).To(Succeed())
		previousCSINode := getCSINode()

		Expect(client.CoreV1().Nodes().Delete(ctx, nodeName, metav1.DeleteOptions{})).To(Succeed())
		node := createNode()

		// The environment does not run the garbage collector, hence the deletion is emulated.
		go func() {
			defer GinkgoRecover()
			time.Sleep(3 * time.Second)
			Expect(client.StorageV1().CSINodes().Delete(ctx, nodeName, metav1.DeleteOptions{})).To(Succeed())
		}()

		Expect(nodeProvider.EnsureCSINode(ctx)).To(Succeed())

		csiNode := getCSINode()
		Expect(csiNode).ToNot(BeNil())
		Expect(csiNode.GetResourceVersion()).ToNot(Equal(previousCSINode.GetResourceVersion()))
		Expect(csiNode.OwnerReferences).To(HaveLen(1))
		Expect(csiNode.OwnerReferences[0].UID).To(Equal(node.GetUID()))
		Expect(csiNode.Spec.Drivers).To(BeEmpty())
	})
})
