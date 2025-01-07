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

package options_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoclient "github.com/liqotech/liqo/pkg/client/clientset/versioned"
	liqoclientfake "github.com/liqotech/liqo/pkg/client/clientset/versioned/fake"
	liqoinformers "github.com/liqotech/liqo/pkg/client/informers/externalversions"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("Options", func() {
	hf := func(options.Keyer, ...options.EventFilter) cache.ResourceEventHandler {
		return cache.ResourceEventHandlerFuncs{}
	}

	Describe("The New function", func() {
		var (
			opts, returned *options.ReflectorOpts
			client         kubernetes.Interface
			informer       corev1informers.PodInformer
			broadcaster    record.EventBroadcaster
		)

		BeforeEach(func() {
			client = fake.NewSimpleClientset()
			informer = informers.NewSharedInformerFactory(client, 0).Core().V1().Pods()
			broadcaster = record.NewBroadcaster()
		})
		JustBeforeEach(func() { opts = options.New(client, informer) })
		It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
		It("should correctly configure the given fields", func() {
			Expect(opts.LocalClient).To(Equal(client))
			Expect(opts.LocalPodInformer).To(Equal(informer))
		})

		Describe("The WithHandlerFactory function", func() {
			JustBeforeEach(func() { returned = opts.WithHandlerFactory(hf) })

			It("should return a non-nil pointer", func() { Expect(returned).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(returned).To(BeIdenticalTo(opts)) })
			It("should correctly set the handler factory value", func() {
				// Comparing the values returned by the functions, as failed to find any better way.
				Expect(opts.HandlerFactory(nil)).To(Equal(opts.HandlerFactory(nil)))
			})
		})

		Describe("The WithReadinessFunc function", func() {
			JustBeforeEach(func() { returned = opts.WithReadinessFunc(func() bool { return true }) })

			It("should return a non-nil pointer", func() { Expect(returned).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(returned).To(BeIdenticalTo(opts)) })

			Context("when the readiness function returns true", func() {
				JustBeforeEach(func() { returned = opts.WithReadinessFunc(func() bool { return true }) })

				It("the opts readiness function should return true", func() { Expect(returned.Ready()).To(BeTrue()) })
			})

			Context("when the readiness function returns false", func() {
				JustBeforeEach(func() { returned = opts.WithReadinessFunc(func() bool { return false }) })

				It("the opts readiness function should return false", func() { Expect(returned.Ready()).To(BeFalse()) })
			})
		})

		Describe("The WithEventBroadcaster function", func() {
			JustBeforeEach(func() { opts = opts.WithEventBroadcaster(broadcaster) })

			It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(opts).To(BeIdenticalTo(opts)) })
			It("should correctly set the event broadcaster value", func() { Expect(opts.EventBroadcaster).To(BeIdenticalTo(broadcaster)) })
		})
	})

	Describe("The NewNamespaced function", func() {
		var opts *options.NamespacedOpts

		JustBeforeEach(func() { opts = options.NewNamespaced() })
		It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
		It("should leave all fields unset", func() {
			Expect(opts.LocalNamespace).To(BeEmpty())
			Expect(opts.RemoteNamespace).To(BeEmpty())
			Expect(opts.LocalClient).To(BeNil())
			Expect(opts.LocalLiqoClient).To(BeNil())
			Expect(opts.LocalFactory).To(BeNil())
			Expect(opts.LocalLiqoFactory).To(BeNil())
			Expect(opts.RemoteClient).To(BeNil())
			Expect(opts.RemoteLiqoClient).To(BeNil())
			Expect(opts.RemoteFactory).To(BeNil())
			Expect(opts.RemoteLiqoFactory).To(BeNil())
			Expect(opts.EventBroadcaster).To(BeNil())
			Expect(opts.HandlerFactory).To(BeNil())
			Expect(opts.Ready).To(BeNil())
			Expect(opts.ReflectionType).To(BeEmpty())
			Expect(opts.ForgingOpts).To(BeNil())
		})
	})

	Describe("The With functions of NamespacedOpts", func() {
		const namespace = "namespace"

		var (
			original, opts *options.NamespacedOpts

			client         kubernetes.Interface
			liqoClient     liqoclient.Interface
			factory        informers.SharedInformerFactory
			liqoFactory    liqoinformers.SharedInformerFactory
			broadcaster    record.EventBroadcaster
			reflectionType offloadingv1beta1.ReflectionType
			forgingOpts    *forge.ForgingOpts
		)

		BeforeEach(func() {
			client = fake.NewSimpleClientset()
			liqoClient = liqoclientfake.NewSimpleClientset()
			factory = informers.NewSharedInformerFactory(client, 10*time.Hour)
			liqoFactory = liqoinformers.NewSharedInformerFactory(liqoClient, 10*time.Hour)
			broadcaster = record.NewBroadcaster()
			reflectionType = offloadingv1beta1.CustomLiqo
			forgingOpts = &forge.ForgingOpts{}
		})

		JustBeforeEach(func() { original = options.NewNamespaced() })

		Describe("The WithLocal function", func() {
			JustBeforeEach(func() { opts = original.WithLocal(namespace, client, factory) })

			It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(opts).To(BeIdenticalTo(original)) })
			It("should correctly set the local namespace value", func() { Expect(opts.LocalNamespace).To(BeIdenticalTo(namespace)) })
			It("should correctly set the local client value", func() { Expect(opts.LocalClient).To(BeIdenticalTo(client)) })
			It("should correctly set the local factory value", func() { Expect(opts.LocalFactory).To(BeIdenticalTo(factory)) })
			It("should leave the other fields unset", func() {
				Expect(opts.RemoteNamespace).To(BeEmpty())
				Expect(opts.LocalLiqoClient).To(BeNil())
				Expect(opts.LocalLiqoFactory).To(BeNil())
				Expect(opts.RemoteClient).To(BeNil())
				Expect(opts.RemoteLiqoClient).To(BeNil())
				Expect(opts.RemoteFactory).To(BeNil())
				Expect(opts.RemoteLiqoFactory).To(BeNil())
				Expect(opts.EventBroadcaster).To(BeNil())
				Expect(opts.HandlerFactory).To(BeNil())
				Expect(opts.Ready).To(BeNil())
				Expect(opts.ReflectionType).To(BeEmpty())
				Expect(opts.ForgingOpts).To(BeNil())
			})
		})

		Describe("The WithLiqoLocal function", func() {
			JustBeforeEach(func() { opts = original.WithLiqoLocal(liqoClient, liqoFactory) })

			It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(opts).To(BeIdenticalTo(original)) })
			It("should correctly set the local liqo client value", func() { Expect(opts.LocalLiqoClient).To(BeIdenticalTo(liqoClient)) })
			It("should correctly set the local liqo factory value", func() { Expect(opts.LocalLiqoFactory).To(BeIdenticalTo(liqoFactory)) })
			It("should leave the other fields unset", func() {
				Expect(opts.LocalNamespace).To(BeEmpty())
				Expect(opts.RemoteNamespace).To(BeEmpty())
				Expect(opts.LocalClient).To(BeNil())
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.RemoteClient).To(BeNil())
				Expect(opts.RemoteLiqoClient).To(BeNil())
				Expect(opts.RemoteFactory).To(BeNil())
				Expect(opts.RemoteLiqoFactory).To(BeNil())
				Expect(opts.EventBroadcaster).To(BeNil())
				Expect(opts.HandlerFactory).To(BeNil())
				Expect(opts.Ready).To(BeNil())
				Expect(opts.ReflectionType).To(BeEmpty())
				Expect(opts.ForgingOpts).To(BeNil())
			})
		})

		Describe("The WithRemote function", func() {
			JustBeforeEach(func() { opts = original.WithRemote(namespace, client, factory) })

			It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(opts).To(BeIdenticalTo(original)) })
			It("should correctly set the remote namespace value", func() { Expect(opts.RemoteNamespace).To(BeIdenticalTo(namespace)) })
			It("should correctly set the remote client value", func() { Expect(opts.RemoteClient).To(BeIdenticalTo(client)) })
			It("should correctly set the remote factory value", func() { Expect(opts.RemoteFactory).To(BeIdenticalTo(factory)) })
			It("should leave the other fields unset", func() {
				Expect(opts.LocalNamespace).To(BeEmpty())
				Expect(opts.LocalClient).To(BeNil())
				Expect(opts.LocalLiqoClient).To(BeNil())
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.LocalLiqoFactory).To(BeNil())
				Expect(opts.RemoteLiqoClient).To(BeNil())
				Expect(opts.RemoteLiqoFactory).To(BeNil())
				Expect(opts.EventBroadcaster).To(BeNil())
				Expect(opts.HandlerFactory).To(BeNil())
				Expect(opts.Ready).To(BeNil())
				Expect(opts.ReflectionType).To(BeEmpty())
				Expect(opts.ForgingOpts).To(BeNil())
			})
		})

		Describe("The WithLiqoRemote function", func() {
			JustBeforeEach(func() { opts = original.WithLiqoRemote(liqoClient, liqoFactory) })

			It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(opts).To(BeIdenticalTo(original)) })
			It("should correctly set the remote liqo client value", func() { Expect(opts.RemoteLiqoClient).To(BeIdenticalTo(liqoClient)) })
			It("should correctly set the remote liqo factory value", func() { Expect(opts.RemoteLiqoFactory).To(BeIdenticalTo(liqoFactory)) })
			It("should leave the other fields unset", func() {
				Expect(opts.LocalNamespace).To(BeEmpty())
				Expect(opts.RemoteNamespace).To(BeEmpty())
				Expect(opts.LocalClient).To(BeNil())
				Expect(opts.LocalLiqoClient).To(BeNil())
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.LocalLiqoFactory).To(BeNil())
				Expect(opts.RemoteClient).To(BeNil())
				Expect(opts.RemoteFactory).To(BeNil())
				Expect(opts.EventBroadcaster).To(BeNil())
				Expect(opts.HandlerFactory).To(BeNil())
				Expect(opts.Ready).To(BeNil())
				Expect(opts.ReflectionType).To(BeEmpty())
				Expect(opts.ForgingOpts).To(BeNil())
			})
		})

		Describe("The WithHandlerFactory function", func() {
			JustBeforeEach(func() { opts = original.WithHandlerFactory(hf) })

			It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(opts).To(BeIdenticalTo(original)) })
			It("should correctly set the handler factory value", func() {
				// Comparing the values returned by the functions, as failed to find any better way.
				Expect(opts.HandlerFactory(nil)).To(Equal(opts.HandlerFactory(nil)))
			})
			It("should leave the other fields unset", func() {
				Expect(opts.LocalNamespace).To(BeEmpty())
				Expect(opts.RemoteNamespace).To(BeEmpty())
				Expect(opts.LocalClient).To(BeNil())
				Expect(opts.LocalLiqoClient).To(BeNil())
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.LocalLiqoFactory).To(BeNil())
				Expect(opts.RemoteClient).To(BeNil())
				Expect(opts.RemoteLiqoClient).To(BeNil())
				Expect(opts.RemoteFactory).To(BeNil())
				Expect(opts.RemoteLiqoFactory).To(BeNil())
				Expect(opts.EventBroadcaster).To(BeNil())
				Expect(opts.Ready).To(BeNil())
				Expect(opts.ReflectionType).To(BeEmpty())
				Expect(opts.ForgingOpts).To(BeNil())
			})
		})

		Describe("The WithReadinessFunc function", func() {
			JustBeforeEach(func() { opts = original.WithReadinessFunc(func() bool { return true }) })

			It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(opts).To(BeIdenticalTo(original)) })
			It("should correctly set the ready value", func() {
				Expect(opts.Ready()).To(BeTrue())
			})
			It("should leave the other fields unset", func() {
				Expect(opts.LocalNamespace).To(BeEmpty())
				Expect(opts.RemoteNamespace).To(BeEmpty())
				Expect(opts.LocalClient).To(BeNil())
				Expect(opts.LocalLiqoClient).To(BeNil())
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.LocalLiqoFactory).To(BeNil())
				Expect(opts.RemoteClient).To(BeNil())
				Expect(opts.RemoteLiqoClient).To(BeNil())
				Expect(opts.RemoteFactory).To(BeNil())
				Expect(opts.RemoteLiqoFactory).To(BeNil())
				Expect(opts.EventBroadcaster).To(BeNil())
				Expect(opts.HandlerFactory).To(BeNil())
				Expect(opts.ReflectionType).To(BeEmpty())
				Expect(opts.ForgingOpts).To(BeNil())
			})
		})

		Describe("The WithEventBroadcaster function", func() {
			JustBeforeEach(func() { opts = original.WithEventBroadcaster(broadcaster) })

			It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(opts).To(BeIdenticalTo(original)) })
			It("should correctly set the event broadcaster value", func() { Expect(opts.EventBroadcaster).To(BeIdenticalTo(broadcaster)) })
			It("should leave the other fields unset", func() {
				Expect(opts.LocalNamespace).To(BeEmpty())
				Expect(opts.RemoteNamespace).To(BeEmpty())
				Expect(opts.LocalClient).To(BeNil())
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.LocalLiqoClient).To(BeNil())
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.RemoteClient).To(BeNil())
				Expect(opts.RemoteLiqoClient).To(BeNil())
				Expect(opts.RemoteFactory).To(BeNil())
				Expect(opts.RemoteLiqoFactory).To(BeNil())
				Expect(opts.HandlerFactory).To(BeNil())
				Expect(opts.Ready).To(BeNil())
				Expect(opts.ReflectionType).To(BeEmpty())
				Expect(opts.ForgingOpts).To(BeNil())
			})
		})

		Describe("The WithReflectionType function", func() {
			JustBeforeEach(func() { opts = original.WithReflectionType(reflectionType) })

			It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(opts).To(BeIdenticalTo(original)) })
			It("should correctly set the reflection type value", func() { Expect(opts.ReflectionType).To(Equal(reflectionType)) })
			It("should leave the other fields unset", func() {
				Expect(opts.LocalNamespace).To(BeEmpty())
				Expect(opts.RemoteNamespace).To(BeEmpty())
				Expect(opts.LocalClient).To(BeNil())
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.LocalLiqoClient).To(BeNil())
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.RemoteClient).To(BeNil())
				Expect(opts.RemoteLiqoClient).To(BeNil())
				Expect(opts.RemoteFactory).To(BeNil())
				Expect(opts.RemoteLiqoFactory).To(BeNil())
				Expect(opts.EventBroadcaster).To(BeNil())
				Expect(opts.HandlerFactory).To(BeNil())
				Expect(opts.Ready).To(BeNil())
				Expect(opts.ForgingOpts).To(BeNil())
			})
		})

		Describe("The WithForgingOpts function", func() {
			JustBeforeEach(func() { opts = original.WithForgingOpts(forgingOpts) })

			It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(opts).To(BeIdenticalTo(original)) })
			It("should correctly set the forging options value", func() { Expect(opts.ForgingOpts).To(BeIdenticalTo(forgingOpts)) })
			It("should leave the other fields unset", func() {
				Expect(opts.LocalNamespace).To(BeEmpty())
				Expect(opts.RemoteNamespace).To(BeEmpty())
				Expect(opts.LocalClient).To(BeNil())
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.LocalLiqoClient).To(BeNil())
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.RemoteClient).To(BeNil())
				Expect(opts.RemoteLiqoClient).To(BeNil())
				Expect(opts.RemoteFactory).To(BeNil())
				Expect(opts.RemoteLiqoFactory).To(BeNil())
				Expect(opts.HandlerFactory).To(BeNil())
				Expect(opts.Ready).To(BeNil())
				Expect(opts.EventBroadcaster).To(BeNil())
				Expect(opts.ReflectionType).To(BeEmpty())
			})
		})
	})
})
