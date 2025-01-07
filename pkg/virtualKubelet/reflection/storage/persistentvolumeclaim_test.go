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

package storage

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("reflector methods", func() {

	var err error

	Context("Handle method", func() {

		Context("storage disabled", func() {

			JustBeforeEach(func() {
				reflector.storageEnabled = false
			})

			When("the remote PVC does not exist", func() {
				JustBeforeEach(func() {
					err = reflector.Handle(ctx, remotePvcName2)
				})

				It("should not create the remote PVC", func() {
					Expect(err).ToNot(HaveOccurred())

					_, err = k8sClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).Get(ctx, remotePvcName2, metav1.GetOptions{})
					Expect(err).To(BeNotFound())
				})

				It("should remove the node annotation triggering rescheduling", func() {
					Expect(err).ToNot(HaveOccurred())

					virtualPvc, err := k8sClient.CoreV1().PersistentVolumeClaims(LocalNamespace).Get(ctx, remotePvcName2, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					_, found := virtualPvc.Annotations[annSelectedNode]
					Expect(found).To(BeFalse())
				})
			})

		})

		Context("storage enabled", func() {

			JustBeforeEach(func() {
				reflector.storageEnabled = true
			})

			When("the remote PVC does not exist", func() {

				var (
					localPvc *corev1.PersistentVolumeClaim
				)

				JustBeforeEach(func() {
					err = reflector.Handle(ctx, remotePvcName)
				})

				It("should create the remote PVC", func() {
					Expect(err).ToNot(HaveOccurred())

					offloadedPvc, err := k8sClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).Get(ctx, remotePvcName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(offloadedPvc).ToNot(BeNil())

					Expect(offloadedPvc.Labels).ToNot(HaveKey(FakeNotReflectedLabelKey))
					Expect(offloadedPvc.Annotations).ToNot(HaveKey(FakeNotReflectedAnnotKey))

					Expect(offloadedPvc.Spec.StorageClassName).To(PointTo(Equal(RealRemoteStorageClassName)))
				})

				It("should update the local PVC", func() {
					Expect(err).ToNot(HaveOccurred())

					localPvc, err = k8sClient.CoreV1().PersistentVolumeClaims(LocalNamespace).Get(ctx, remotePvcName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(localPvc.Spec.VolumeName).ToNot(BeEmpty())
				})

				It("should create the local PV", func() {
					Expect(err).ToNot(HaveOccurred())

					localPv, err := k8sClient.CoreV1().PersistentVolumes().Get(ctx, localPvc.Spec.VolumeName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(localPv).ToNot(BeNil())
				})
			})

			When("the PVC is not on the virtual node", func() {
				JustBeforeEach(func() {
					// the function has to succeed, we have not to reenqueue this item
					err = reflector.Handle(ctx, localPvcName)
				})

				It("should not create the remote PVC", func() {
					Expect(err).ToNot(HaveOccurred())

					_, err = k8sClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).Get(ctx, localPvcName, metav1.GetOptions{})
					Expect(err).To(BeNotFound())
				})
			})

		})

	})

})
