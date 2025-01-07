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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("controller methods", func() {

	Context("shouldProvision method", func() {

		type shouldProvisionTestcase struct {
			claim          *corev1.PersistentVolumeClaim
			expectedShould OmegaMatcher
		}

		DescribeTable("shouldProvision table",
			func(c shouldProvisionTestcase) {
				should, err := reflector.shouldProvision(c.claim)
				Expect(err).ToNot(HaveOccurred())
				Expect(should).To(c.expectedShould)
			},

			Entry("volume already defined", shouldProvisionTestcase{
				claim: &corev1.PersistentVolumeClaim{
					Spec: corev1.PersistentVolumeClaimSpec{
						VolumeName: "test",
					},
				},
				expectedShould: BeFalse(),
			}),

			Entry("PVC with wrong provisioner", shouldProvisionTestcase{
				claim: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annStorageProvisioner: "other-provisioner",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{},
				},
				expectedShould: BeFalse(),
			}),

			Entry("PVC with wrong storage class", shouldProvisionTestcase{
				claim: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annStorageProvisioner: consts.StorageProvisionerName,
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: pointer.String("other-class"),
					},
				},
				expectedShould: BeFalse(),
			}),

			Entry("correct PVC", shouldProvisionTestcase{
				claim: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annStorageProvisioner: consts.StorageProvisionerName,
							annSelectedNode:       "node-1",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: pointer.String(VirtualStorageClassName),
					},
				},
				expectedShould: BeTrue(),
			}),
		)

	})

})
