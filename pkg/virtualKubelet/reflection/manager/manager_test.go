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

package manager

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	reflectionfake "github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic/fake"
)

var _ = Describe("Manager tests", func() {
	const (
		localNamespace  = "local"
		remoteNamespace = "remote"
	)

	var (
		mgr          Manager
		localClient  kubernetes.Interface
		remoteClient kubernetes.Interface

		ctx    context.Context
		cancel context.CancelFunc
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		localClient = fake.NewSimpleClientset()
		remoteClient = fake.NewSimpleClientset()
	})
	AfterEach(func() { cancel() })

	JustBeforeEach(func() { mgr = New(localClient, remoteClient, 1*time.Hour) })

	Context("a new manager is created", func() {
		It("should return a non nil manager", func() { Expect(mgr).ToNot(BeNil()) })
		It("should correctly populate the manager fields", func() {
			Expect(mgr.(*manager).local).To(Equal(localClient))
			Expect(mgr.(*manager).remote).To(Equal(remoteClient))
			Expect(mgr.(*manager).resync).To(Equal(1 * time.Hour))

			Expect(mgr.(*manager).reflectors).ToNot(BeNil())
			Expect(mgr.(*manager).localPodInformerFactory).ToNot(BeNil())

			Expect(mgr.(*manager).started).To(BeFalse())
			Expect(mgr.(*manager).stop).ToNot(BeNil())
		})

		Context("a reflector is registered", func() {
			var (
				returned  Manager
				reflector *reflectionfake.Reflector
			)

			BeforeEach(func() { reflector = reflectionfake.NewReflector() })
			JustBeforeEach(func() { returned = mgr.With(reflector) })

			It("should return the receiver manager", func() { Expect(mgr).To(BeIdenticalTo(returned)) })
			It("should correctly add the reflector to the list", func() {
				Expect(mgr.(*manager).reflectors).To(ConsistOf(reflector))
			})

			Context("the manager is started", func() {
				JustBeforeEach(func() { mgr.Start(ctx) })

				It("should set the manager as started", func() { Expect(mgr.(*manager).started).To(BeTrue()) })
				It("should start the registered reflector", func() { Expect(reflector.Started).To(BeTrue()) })
				It("should correctly populate the reflector options", func() {
					Expect(reflector.Opts.LocalClient).To(Equal(localClient))
					Expect(reflector.Opts.LocalPodInformer).ToNot(BeNil())
					Expect(reflector.Opts.HandlerFactory).To(BeNil())
				})
				It("should panic if started twice", func() { Expect(func() { mgr.Start(ctx) }).To(Panic()) })

				Context("a namespace is started", func() {
					JustBeforeEach(func() { mgr.StartNamespace(localNamespace, remoteNamespace) })

					It("should add the stop entry", func() { Expect(mgr.(*manager).stop).To(HaveKey(localNamespace)) })
					It("should start the registered reflector", func() { Expect(reflector.NamespaceStarted).To(HaveKey(localNamespace)) })
					It("should correctly populate the reflector options", func() {
						opts := reflector.NamespaceStarted[localNamespace]
						Expect(opts.LocalNamespace).To(Equal(localNamespace))
						Expect(opts.LocalClient).To(Equal(localClient))
						Expect(opts.LocalFactory).ToNot(BeNil())
						Expect(opts.RemoteNamespace).To(Equal(remoteNamespace))
						Expect(opts.RemoteClient).To(Equal(remoteClient))
						Expect(opts.RemoteFactory).ToNot(BeNil())
						Expect(opts.Ready).ToNot(BeNil())
						Expect(opts.HandlerFactory).To(BeNil())
					})
					It("should eventually mark the namespace as ready", func() {
						Eventually(reflector.NamespaceStarted[localNamespace].Ready).Should(BeTrue())
					})

					Context("the same namespace is stopped", func() {
						JustBeforeEach(func() { mgr.StopNamespace(localNamespace, remoteNamespace) })

						It("should remote the stop entry", func() { Expect(mgr.(*manager).stop).ToNot(HaveKey(localNamespace)) })
						It("should stop the registered reflector", func() {
							Expect(reflector.NamespaceStopped).To(HaveKeyWithValue(localNamespace, remoteNamespace))
						})
					})
				})
			})
		})
	})
})
