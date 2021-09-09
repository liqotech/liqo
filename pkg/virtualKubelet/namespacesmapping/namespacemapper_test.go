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

package namespacesmapping

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	vkalpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (
	homeClusterID             = "test-cluster1-trert"
	foreignClusterID          = "test-cluster2-2er2r"
	otherForeignClusterID     = "test-cluster2-2er2r"
	localValidNamespace       = "local-ns1"
	localNotExistingNamespace = "local-ns-not-existing"
	localNotReadyNamespace    = "local-ns-not-ready"
	foreignValidNamespace     = "remote-ns1"
	foreignNotFoundNamespace  = "remote-ns-not-existing"
	foreignNotReadyNamespace  = "remote-ns-not-ready"
	testNamespaceSingle       = "test-namespace-mapper-single"
)

var namespaceMap = vkalpha1.NamespaceMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      testNamespaceSingle,
		Labels:    map[string]string{liqoconst.RemoteClusterID: otherForeignClusterID},
		Namespace: testNamespaceSingle,
	},
	Spec: vkalpha1.NamespaceMapSpec{
		DesiredMapping: map[string]string{
			localValidNamespace:    foreignValidNamespace,
			localNotReadyNamespace: foreignNotReadyNamespace,
		},
	},
	Status: vkalpha1.NamespaceMapStatus{
		CurrentMapping: map[string]vkalpha1.RemoteNamespaceStatus{
			localValidNamespace: {
				RemoteNamespace: foreignValidNamespace,
				Phase:           vkalpha1.MappingAccepted,
			},
			localNotReadyNamespace: {
				RemoteNamespace: foreignNotReadyNamespace,
				Phase:           vkalpha1.MappingCreationLoopBackOff,
			},
		},
	},
}

var _ = Describe("Test that NamespaceMapper", func() {
	Context("correctly initializes the namespaceMapper module", func() {
		m := NamespaceMapper{
			homeClusterID:    homeClusterID,
			foreignClusterID: foreignClusterID,
			namespaceReadyMapCache: namespaceReadyMapCache{
				RWMutex: sync.RWMutex{},
				mappings: map[string]string{
					localValidNamespace: foreignValidNamespace,
				},
			},
		}
		DescribeTable("and handles correct/not-found namespace translations",
			func(desiredNamespace, translatedNS string, mappingFunction func(string) (string, error),
				expectedError error) {
				nattedNS, err := mappingFunction(desiredNamespace)
				if expectedError == nil {
					Expect(err).NotTo(HaveOccurred())
				} else {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(BeEquivalentTo(expectedError.Error()))
				}
				Expect(nattedNS).To(BeEquivalentTo(translatedNS))
			},

			// this entry should be taken by the operator, and it should set the phase and the virtual-kubelet deployment accordingly.
			Entry("Correct HomeToForeign translation", localValidNamespace, foreignValidNamespace, m.HomeToForeignNamespace, nil),
			Entry("Incorrect HomeToForeign translation", localNotExistingNamespace, namespaceMapEntryNotAvailable, m.HomeToForeignNamespace,
				&namespaceNotAvailable{
					namespaceName: localNotExistingNamespace,
				}),
			// Entries to test foreign to local translations
			Entry("", foreignValidNamespace, localValidNamespace, m.ForeignToLocalNamespace, nil),
			Entry("", foreignNotFoundNamespace, namespaceMapEntryNotAvailable, m.ForeignToLocalNamespace,
				&namespaceNotAvailable{
					namespaceName: foreignNotFoundNamespace,
				}),
		)
	})
	Context("correctly initializes the namespaceMapper module", func() {
		m := NamespaceMapper{
			homeClusterID:           homeClusterID,
			foreignClusterID:        foreignClusterID,
			namespace:               testNamespaceSingle,
			startOutgoingReflection: make(chan string, 100),
			startIncomingReflection: make(chan string, 100),
			stopIncomingReflection:  make(chan string, 100),
			stopOutgoingReflection:  make(chan string, 100),
			startMapper:             make(chan struct{}, 100),
			stopMapper:              make(chan struct{}, 100),
			restartReady:            make(chan struct{}, 100),
		}
		namespace := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespaceSingle,
			},
		}
		It("Check NamespaceMap reconciliation", func() {
			Expect(m.init(ctx, cfg)).NotTo(HaveOccurred())

			By("fails when no NamespaceMaps exists", func() {
				Expect(k8sClient.Create(ctx, &namespace)).NotTo(HaveOccurred())
				Expect(m.checkMapUniqueness()).To(BeTrue())
			})

			By("Fill the cache when a namespaceMap is created", func() {
				Expect(k8sClient.Create(ctx, &namespaceMap)).NotTo(HaveOccurred())
				Expect(m.checkMapUniqueness()).To(BeTrue())
				Eventually(func() bool {
					return m.namespaceReadyMapCache.mappings[localValidNamespace] == foreignValidNamespace
				}, 60*time.Second, 3*time.Second).Should(Equal(true))
			})
			val, err := m.HomeToForeignNamespace(foreignNotReadyNamespace)
			Expect(val).To(BeEquivalentTo(namespaceMapEntryNotAvailable))
			Expect(err).To(HaveOccurred())

			By("Update the internal cache when a namespaceMap with a new Ready namespace", func() {
				var namespaceMap2 vkalpha1.NamespaceMap
				err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
					err = k8sClient.Get(ctx, types.NamespacedName{
						Name:      testNamespaceSingle,
						Namespace: testNamespaceSingle,
					}, &namespaceMap2)
					if err != nil {
						return err
					}
					namespaceMap2.Status.CurrentMapping[localNotReadyNamespace] = vkalpha1.RemoteNamespaceStatus{
						RemoteNamespace: foreignNotReadyNamespace,
						Phase:           vkalpha1.MappingAccepted,
					}
					err = k8sClient.Update(ctx, &namespaceMap2)
					if err != nil {
						return err
					}
					return nil
				})
				Eventually(func() bool {
					return m.namespaceReadyMapCache.mappings[localNotReadyNamespace] == foreignNotReadyNamespace
				}, 10*time.Second, 1*time.Second).Should(Equal(true))
				val, err = m.HomeToForeignNamespace(localValidNamespace)
				Expect(val).To(BeEquivalentTo(foreignValidNamespace))
				Expect(err).NotTo(HaveOccurred())
			})

			By("Update the internal cache when a namespaceMap with a NotReady namespace", func() {
				var namespaceMap2 vkalpha1.NamespaceMap
				err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
					err = k8sClient.Get(ctx, types.NamespacedName{
						Name:      testNamespaceSingle,
						Namespace: testNamespaceSingle,
					}, &namespaceMap2)
					if err != nil {
						return err
					}
					namespaceMap2.Status.CurrentMapping[localNotReadyNamespace] = vkalpha1.RemoteNamespaceStatus{
						RemoteNamespace: foreignNotReadyNamespace,
						Phase:           vkalpha1.MappingCreationLoopBackOff,
					}
					err = k8sClient.Update(ctx, &namespaceMap2)
					if err != nil {
						return err
					}
					return nil
				})
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() bool {
					return m.namespaceReadyMapCache.mappings[localNotReadyNamespace] == foreignNotReadyNamespace
				}, 30*time.Second, 1*time.Second).Should(Equal(true))
				val, err = m.HomeToForeignNamespace(localValidNamespace)
				Expect(val).To(BeEquivalentTo(foreignValidNamespace))
				Expect(err).NotTo(HaveOccurred())
			})

			By("Deletes the internal cache when a namespaceMap is destroyed")
			Expect(k8sClient.Delete(ctx, &namespaceMap)).NotTo(HaveOccurred())
			Eventually(func() bool {
				return len(m.namespaceReadyMapCache.mappings) == 2
			}, 10*time.Second, 1*time.Second).Should(Equal(true))
		})

	})
})
