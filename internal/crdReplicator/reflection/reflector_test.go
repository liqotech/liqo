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

package reflection

import (
	"context"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/resources"
	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("Reflector tests", func() {
	const (
		localNamespace  = "foo"
		remoteNamespace = "bar"
		localClusterID  = "local-id"
		remoteClusterID = "remote-id"
		workers         = 1
	)

	var (
		ctx    context.Context
		cancel context.CancelFunc

		gvr schema.GroupVersionResource
		res resources.Resource

		manager   *Manager
		reflector *Reflector

		local, remote dynamic.Interface
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(offloadingv1beta1.AddToScheme(scheme))

		ctx, cancel = context.WithTimeout(context.Background(), 1*time.Second)
		gvr = offloadingv1beta1.NamespaceMapGroupVersionResource
		res = resources.Resource{GroupVersionResource: gvr, Ownership: consts.OwnershipLocal}

		local = fake.NewSimpleDynamicClient(scheme)
		remote = fake.NewSimpleDynamicClient(scheme)

		manager = NewManager(local, localClusterID, workers, 0)
		manager.Start(ctx, []resources.Resource{res})
		reflector = manager.NewForRemote(remote, remoteClusterID, localNamespace, remoteNamespace, "")
	})

	AfterEach(func() { cancel() })

	Describe("the StartForResource function", func() {
		const name = "network-config"

		BeforeEach(func() { reflector.StartForResource(ctx, &res) })

		CreateLocalObject := func() {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(offloadingv1beta1.SchemeGroupVersion.WithKind("NamespaceMap"))
			obj.SetNamespace(localNamespace)
			obj.SetName(name)
			obj.SetLabels(map[string]string{
				consts.ReplicationRequestedLabel:   strconv.FormatBool(true),
				consts.ReplicationDestinationLabel: remoteClusterID,
			})
			_, err := local.Resource(gvr).Namespace(localNamespace).Create(ctx, obj, v1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		}

		CreateRemoteObject := func() {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(offloadingv1beta1.SchemeGroupVersion.WithKind("NamespaceMap"))
			obj.SetNamespace(remoteNamespace)
			obj.SetName(name)
			obj.SetLabels(map[string]string{
				consts.ReplicationStatusLabel: strconv.FormatBool(true),
				consts.ReplicationOriginLabel: localClusterID,
			})
			_, err := remote.Resource(gvr).Namespace(remoteNamespace).Create(ctx, obj, v1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		}

		It("should correctly construct the reflected resource object", func() {
			Expect(reflector.resources).To(HaveKey(gvr))
			Expect(reflector.resources[gvr].gvr).To(Equal(gvr))
			Expect(reflector.resources[gvr].ownership).To(Equal(res.Ownership))
			Expect(reflector.resources[gvr].local).ToNot(BeNil())
			Expect(reflector.resources[gvr].remote).ToNot(BeNil())
			Expect(reflector.resources[gvr].cancel).ToNot(BeNil())
		})

		It("should set the resource reflection as started", func() {
			Expect(reflector.ResourceStarted(&res)).To(BeTrue())
		})

		It("should correctly complete the initialization", func() {
			Expect(reflector.resources).To(HaveKey(gvr))
			Eventually(func() bool { return reflector.resources[gvr].initialized }).Should(BeTrue())
		})

		Describe("the handlers are triggered", func() {
			BeforeEach(func() {
				// Wait for the cache to be completely initialized
				Eventually(func() bool { return reflector.resources[gvr].initialized }).Should(BeTrue())
			})

			ItBody := func() {
				key, shutdown := reflector.workqueue.Get()
				Expect(key).To(Equal(item{gvr: gvr, name: name}))
				Expect(shutdown).To(BeFalse())
			}

			When("a local object is created", func() {
				JustBeforeEach(func() { CreateLocalObject() })
				It("Should be added to the working queue", func() { ItBody() })
			})

			When("a remote object is created", func() {
				JustBeforeEach(func() { CreateRemoteObject() })
				It("Should be added to the working queue", func() { ItBody() })
			})
		})

		Describe("the resource reflection is stopped", func() {
			var err error

			BeforeEach(func() {
				// Wait for the cache to be completely initialized
				Eventually(func() bool { return reflector.resources[gvr].initialized }).Should(BeTrue())
			})

			JustBeforeEach(func() { err = reflector.stopForResource(gvr, false) })

			When("no object is present", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should no longer be active", func() { Expect(reflector.ResourceStarted(&res)).To(BeFalse()) })
				When("a local object is created", func() {
					JustBeforeEach(func() { CreateLocalObject() })
					It("Should not be added to the working queue", func() {
						Consistently(reflector.workqueue.Len()).Should(BeNumerically("==", 0))
					})
				})

				When("a remote object is created", func() {
					JustBeforeEach(func() { CreateRemoteObject() })
					It("Should not be added to the working queue", func() {
						Consistently(reflector.workqueue.Len()).Should(BeNumerically("==", 0))
					})
				})
			})

			WhenBody := func(creator func(), retriever func(*reflectedResource) cache.GenericNamespaceLister) func() {
				return func() {
					BeforeEach(func() {
						creator()

						// Make sure the object gets cached by the lister
						Eventually(func() error {
							_, e := retriever(reflector.resources[gvr]).Get(name)
							return e
						}).Should(Succeed())
					})
					It("should return an error", func() { Expect(err).To(HaveOccurred()) })
				}
			}

			When("a local object is present", WhenBody(CreateLocalObject, func(rr *reflectedResource) cache.GenericNamespaceLister { return rr.local }))
			When("a remote object is present", WhenBody(CreateRemoteObject, func(rr *reflectedResource) cache.GenericNamespaceLister { return rr.remote }))
		})
	})
})
