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

package options_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("Options", func() {
	Describe("The New function", func() {
		var opts *options.ReflectorOpts

		JustBeforeEach(func() { opts = options.New() })
		It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
		It("should leave all fields unset", func() {
			Expect(opts.LocalNamespace).To(BeEmpty())
			Expect(opts.RemoteNamespace).To(BeEmpty())
			Expect(opts.LocalClient).To(BeNil())
			Expect(opts.RemoteClient).To(BeNil())
			Expect(opts.LocalFactory).To(BeNil())
			Expect(opts.RemoteFactory).To(BeNil())
			Expect(opts.HandlerFactory).To(BeNil())
		})
	})

	Describe("The With functions", func() {
		const namespace = "namespace"

		var (
			original, opts *options.ReflectorOpts

			client  kubernetes.Interface
			factory informers.SharedInformerFactory
		)

		BeforeEach(func() {
			client = fake.NewSimpleClientset()
			factory = informers.NewSharedInformerFactory(client, 10*time.Hour)
		})

		JustBeforeEach(func() { original = options.New() })

		Describe("The WithLocal function", func() {
			JustBeforeEach(func() { opts = original.WithLocal(namespace, client, factory) })

			It("should return a non-nil pointer", func() { Expect(opts).ToNot(BeNil()) })
			It("should return the same pointer of the receiver", func() { Expect(opts).To(BeIdenticalTo(original)) })
			It("should correctly set the local namespace value", func() { Expect(opts.LocalNamespace).To(BeIdenticalTo(namespace)) })
			It("should correctly set the local client value", func() { Expect(opts.LocalClient).To(BeIdenticalTo(client)) })
			It("should correctly set the local factory value", func() { Expect(opts.LocalFactory).To(BeIdenticalTo(factory)) })
			It("should leave the other fields unset", func() {
				Expect(opts.RemoteNamespace).To(BeEmpty())
				Expect(opts.RemoteClient).To(BeNil())
				Expect(opts.RemoteFactory).To(BeNil())
				Expect(opts.HandlerFactory).To(BeNil())
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
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.HandlerFactory).To(BeNil())
			})
		})

		Describe("The WithHandlerFactory function", func() {
			hf := func(options.Keyer) cache.ResourceEventHandler { return cache.ResourceEventHandlerFuncs{} }
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
				Expect(opts.RemoteClient).To(BeNil())
				Expect(opts.LocalFactory).To(BeNil())
				Expect(opts.RemoteFactory).To(BeNil())
			})
		})
	})
})
