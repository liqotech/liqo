// Copyright 2019-2021 The Liqo Authors
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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("reflector methods", func() {

	Context("Handle method", func() {

		It("the remote PVC does not exist", func() {
			Expect(reflector.Handle(ctx, remotePvcName)).To(Succeed())

			offloadedPvc, err := k8sClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).Get(ctx, remotePvcName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(offloadedPvc).ToNot(BeNil())

			Expect(offloadedPvc.Spec.StorageClassName).To(PointTo(Equal(RealRemoteStorageClassName)))

			localPvc, err := k8sClient.CoreV1().PersistentVolumeClaims(LocalNamespace).Get(ctx, remotePvcName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(localPvc.Spec.VolumeName).ToNot(BeEmpty())

			localPv, err := k8sClient.CoreV1().PersistentVolumes().Get(ctx, localPvc.Spec.VolumeName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(localPv).ToNot(BeNil())
		})

		It("the PVC is not on the virtual node", func() {
			// the function has to succeed, we have not reenqueue this item
			Expect(reflector.Handle(ctx, localPvcName)).To(Succeed())

			_, err := k8sClient.CoreV1().PersistentVolumeClaims(RemoteNamespace).Get(ctx, localPvcName, metav1.GetOptions{})
			Expect(err).To(BeNotFound())
		})

	})

})
