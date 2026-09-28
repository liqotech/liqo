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

package manager

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	gwclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gwclientfake "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/fake"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	liqoclientfake "github.com/liqotech/liqo/pkg/client/clientset/versioned/fake"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	reflectionfake "github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic/fake"
)

var _ = Describe("Manager tests", func() {
	const (
		localNamespace  = "local"
		remoteNamespace = "remote"
	)

	var (
		mgr              Manager
		localClient      kubernetes.Interface
		remoteClient     kubernetes.Interface
		localLiqoClient  liqoclient.Interface
		remoteLiqoClient liqoclient.Interface
		broadcaster      record.EventBroadcaster
		offloadingPatch  offloadingv1beta1.OffloadingPatch
		forgingOpts      forge.ForgingOpts

		ctx    context.Context
		cancel context.CancelFunc
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		localClient = fake.NewSimpleClientset()
		remoteClient = fake.NewSimpleClientset()
		localLiqoClient = liqoclientfake.NewSimpleClientset()
		remoteLiqoClient = liqoclientfake.NewSimpleClientset()
		broadcaster = record.NewBroadcaster()
		forgingOpts = forge.NewForgingOpts(&offloadingPatch)
	})
	AfterEach(func() { cancel() })

	JustBeforeEach(func() {
		mgr = New(localClient, remoteClient, localLiqoClient, remoteLiqoClient, 1*time.Hour, broadcaster, &forgingOpts)
	})

	Context("a new manager is created", func() {
		It("should return a non nil manager", func() { Expect(mgr).ToNot(BeNil()) })
		It("should correctly populate the manager fields", func() {
			Expect(mgr.(*manager).local).To(Equal(localClient))
			Expect(mgr.(*manager).remote).To(Equal(remoteClient))
			Expect(mgr.(*manager).localLiqo).To(Equal(localLiqoClient))
			Expect(mgr.(*manager).remoteLiqo).To(Equal(remoteLiqoClient))
			Expect(mgr.(*manager).resync).To(Equal(1 * time.Hour))
			Expect(mgr.(*manager).eventBroadcaster).To(Equal(broadcaster))

			Expect(mgr.(*manager).namespaceHandler).To(BeNil())

			Expect(mgr.(*manager).reflectors).ToNot(BeNil())
			Expect(mgr.(*manager).localPodInformerFactory).ToNot(BeNil())

			Expect(mgr.(*manager).started).To(BeFalse())
			Expect(mgr.(*manager).stop).ToNot(BeNil())
			Expect(mgr.(*manager).namespaces).ToNot(BeNil())

			Expect(mgr.(*manager).localGateway).To(BeNil())
			Expect(mgr.(*manager).remoteGateway).To(BeNil())

			Expect(mgr.(*manager).forgingOpts).ToNot(BeNil())
		})

		Context("a NamespaceMapEventHandler is registered", func() {
			var (
				returned Manager
				handler  *fakeNamespaceHandler
			)

			BeforeEach(func() { handler = &fakeNamespaceHandler{} })
			JustBeforeEach(func() { returned = mgr.WithNamespaceHandler(handler) })

			It("should return the receiver manager", func() { Expect(mgr).To(BeIdenticalTo(returned)) })
			It("should correctly add the NamespaceMapEventHandler", func() {
				Expect(mgr.(*manager).namespaceHandler).To(BeIdenticalTo(handler))
			})

			Context("the manager is started", func() {
				JustBeforeEach(func() {
					mgr.Start(ctx)
				})

				It("should start the registered NamespaceMapEventHandler", func() { Expect(handler.StartCalled).To(BeEquivalentTo(1)) })
			})
		})

		Context("a reflector is registered", func() {
			var (
				returned  Manager
				reflector *reflectionfake.Reflector
			)

			BeforeEach(func() { reflector = reflectionfake.NewReflector(false) })
			JustBeforeEach(func() { returned = mgr.With(reflector) })

			It("should return the receiver manager", func() { Expect(mgr).To(BeIdenticalTo(returned)) })
			It("should correctly add the reflector to the list", func() {
				Expect(mgr.(*manager).reflectors).To(ConsistOf(reflector))
			})

			Context("the manager is started", func() {
				JustBeforeEach(func() {
					mgr.WithNamespaceHandler(&fakeNamespaceHandler{})
					mgr.Start(ctx)
				})

				It("should set the manager as started", func() { Expect(mgr.(*manager).started).To(BeTrue()) })
				It("should start the registered reflector", func() { Expect(reflector.Started).To(BeTrue()) })
				It("should correctly populate the reflector options", func() {
					Expect(reflector.Opts.LocalClient).To(Equal(localClient))
					Expect(reflector.Opts.LocalPodInformer).ToNot(BeNil())
					Expect(reflector.Opts.HandlerFactory).To(BeNil())
					Expect(reflector.Opts.Ready).ToNot(BeNil())
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
						Expect(opts.LocalLiqoClient).To(Equal(localLiqoClient))
						Expect(opts.LocalFactory).ToNot(BeNil())
						Expect(opts.LocalLiqoFactory).ToNot(BeNil())
						Expect(opts.RemoteNamespace).To(Equal(remoteNamespace))
						Expect(opts.RemoteClient).To(Equal(remoteClient))
						Expect(opts.RemoteLiqoClient).To(Equal(remoteLiqoClient))
						Expect(opts.RemoteFactory).ToNot(BeNil())
						Expect(opts.RemoteLiqoFactory).ToNot(BeNil())
						Expect(opts.EventBroadcaster).To(Equal(broadcaster))
						Expect(opts.Ready).ToNot(BeNil())
						Expect(opts.HandlerFactory).To(BeNil())
						Expect(opts.ForgingOpts).ToNot(BeNil())
						Expect(opts.NamespaceMapper).ToNot(BeNil())
					})
					It("should not configure the Gateway API parameters", func() {
						opts := reflector.NamespaceStarted[localNamespace]
						Expect(opts.LocalGatewayClient).To(BeNil())
						Expect(opts.LocalGatewayFactory).To(BeNil())
						Expect(opts.RemoteGatewayClient).To(BeNil())
						Expect(opts.RemoteGatewayFactory).To(BeNil())
					})
					It("should eventually mark the namespace as ready", func() {
						Eventually(reflector.NamespaceStarted[localNamespace].Ready).Should(BeTrue())
					})
					It("should map the local namespace to the remote one", func() {
						remote, found := mgr.RemoteNamespaceFor(localNamespace)
						Expect(found).To(BeTrue())
						Expect(remote).To(Equal(remoteNamespace))

						remote, found = reflector.NamespaceStarted[localNamespace].NamespaceMapper(localNamespace)
						Expect(found).To(BeTrue())
						Expect(remote).To(Equal(remoteNamespace))

						_, found = mgr.RemoteNamespaceFor("not-reflected")
						Expect(found).To(BeFalse())
					})

					Context("the same namespace is stopped", func() {
						JustBeforeEach(func() { mgr.StopNamespace(localNamespace, remoteNamespace) })

						It("should remote the stop entry", func() { Expect(mgr.(*manager).stop).ToNot(HaveKey(localNamespace)) })
						It("should stop the registered reflector", func() {
							Expect(reflector.NamespaceStopped).To(HaveKeyWithValue(localNamespace, remoteNamespace))
						})
						It("should remove the namespace mapping", func() {
							_, found := mgr.RemoteNamespaceFor(localNamespace)
							Expect(found).To(BeFalse())
						})
					})
				})
			})
		})

		Context("the Gateway API clients are configured", func() {
			var (
				returned  Manager
				reflector *reflectionfake.Reflector

				localGatewayClient  gwclient.Interface
				remoteGatewayClient gwclient.Interface
			)

			BeforeEach(func() {
				reflector = reflectionfake.NewReflector(false)
				localGatewayClient = gwclientfake.NewClientset()
				remoteGatewayClient = gwclientfake.NewClientset()
			})
			JustBeforeEach(func() { returned = mgr.WithGatewayAPI(localGatewayClient, remoteGatewayClient).With(reflector) })

			It("should return the receiver manager", func() { Expect(mgr).To(BeIdenticalTo(returned)) })
			It("should correctly set the Gateway API clients", func() {
				Expect(mgr.(*manager).localGateway).To(BeIdenticalTo(localGatewayClient))
				Expect(mgr.(*manager).remoteGateway).To(BeIdenticalTo(remoteGatewayClient))
			})
			It("should create the cluster-wide local informer factory", func() {
				Expect(mgr.(*manager).localGatewayFactory).ToNot(BeNil())
			})

			Context("a namespace is started", func() {
				JustBeforeEach(func() {
					mgr.WithNamespaceHandler(&fakeNamespaceHandler{})
					mgr.Start(ctx)
					mgr.StartNamespace(localNamespace, remoteNamespace)
				})

				It("should panic if configured after start", func() {
					Expect(func() { mgr.WithGatewayAPI(localGatewayClient, remoteGatewayClient) }).To(Panic())
				})
				It("should correctly populate the reflector options", func() {
					Expect(reflector.Opts.LocalGatewayFactory).To(BeIdenticalTo(mgr.(*manager).localGatewayFactory))
					Expect(reflector.Opts.NamespaceMapper).ToNot(BeNil())
				})
				It("should correctly populate the Gateway API parameters", func() {
					opts := reflector.NamespaceStarted[localNamespace]
					Expect(opts.LocalGatewayClient).To(BeIdenticalTo(localGatewayClient))
					Expect(opts.LocalGatewayFactory).To(BeIdenticalTo(mgr.(*manager).localGatewayFactory))
					Expect(opts.RemoteGatewayClient).To(BeIdenticalTo(remoteGatewayClient))
					Expect(opts.RemoteGatewayFactory).ToNot(BeNil())
				})
				It("should eventually mark the namespace as ready", func() {
					Eventually(reflector.NamespaceStarted[localNamespace].Ready).Should(BeTrue())
				})

				When("the remote client is not configured", func() {
					BeforeEach(func() { remoteGatewayClient = nil })

					It("should configure the local Gateway API parameters only", func() {
						opts := reflector.NamespaceStarted[localNamespace]
						Expect(opts.LocalGatewayClient).To(BeIdenticalTo(localGatewayClient))
						Expect(opts.LocalGatewayFactory).ToNot(BeNil())
						Expect(opts.RemoteGatewayClient).To(BeNil())
						Expect(opts.RemoteGatewayFactory).To(BeNil())
					})
				})
			})
		})
	})
})

// fakeNamespaceHandler implements a fake NamespaceHandler for testing purpouses.
type fakeNamespaceHandler struct {
	StartCalled int
}

// Start is the fake Start method.
func (nh *fakeNamespaceHandler) Start(_ context.Context, _ NamespaceStartStopper) {
	nh.StartCalled++
}
