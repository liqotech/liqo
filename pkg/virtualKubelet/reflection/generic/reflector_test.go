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

package generic

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	reflectionfake "github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic/fake"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("Reflector tests", func() {
	Describe("the reflector methods", func() {
		const (
			localNamespace  = "local"
			remoteNamespace = "remote"
			reflectorName   = "reflector"
		)

		var (
			rfl   manager.Reflector
			nsrfl *reflectionfake.NamespacedReflector
		)

		NewFakeNamespacedReflector := func(opts *options.ReflectorOpts) manager.NamespacedReflector {
			nsrfl = reflectionfake.NewNamespacedReflector(opts)
			return nsrfl
		}

		Context("a new reflector is created", func() {
			JustBeforeEach(func() { rfl = NewReflector(reflectorName, NewFakeNamespacedReflector, 10) })
			It("should return a non nil reflector", func() { Expect(rfl).ToNot(BeNil()) })
			It("should correctly populate the reflector fields", func() {
				Expect(rfl.(*reflector).name).To(BeIdenticalTo(reflectorName))
				Expect(rfl.(*reflector).workers).To(BeNumerically("==", 10))

				Expect(rfl.(*reflector).workqueue).ToNot(BeNil())

				Expect(rfl.(*reflector).reflectors).ToNot(BeNil())
				Expect(rfl.(*reflector).factory).ToNot(BeNil())
			})

			Context("a new namespace is started", func() {
				var opts options.ReflectorOpts

				BeforeEach(func() { opts = options.ReflectorOpts{LocalNamespace: localNamespace, RemoteNamespace: remoteNamespace} })
				JustBeforeEach(func() { rfl.StartNamespace(&opts) })

				It("should create a new namespaced reflector", func() {
					Expect(rfl.(*reflector).reflectors).To(HaveKeyWithValue(localNamespace, nsrfl))
				})
				It("should correctly propagate the reflector options", func() {
					Expect(nsrfl.Opts.LocalNamespace).To(Equal(localNamespace))
					Expect(nsrfl.Opts.RemoteNamespace).To(Equal(remoteNamespace))
					Expect(opts.HandlerFactory).ToNot(BeNil())
				})

				Context("the same namespace is stopped", func() {
					JustBeforeEach(func() { rfl.StopNamespace(localNamespace, remoteNamespace) })
					It("should remove the namespaced reflector", func() {
						Expect(rfl.(*reflector).reflectors).ToNot(HaveKeyWithValue(localNamespace, nsrfl))
					})
				})

				Context("a namespace is set as ready", func() {
					var namespace string

					JustBeforeEach(func() { rfl.SetNamespaceReady(namespace) })
					When("the namespace exists", func() {
						BeforeEach(func() { namespace = localNamespace })
						It("should set the namespaced reflector as ready", func() { Expect(nsrfl.Ready()).To(BeTrue()) })
					})
					When("the namespace does not exist", func() {
						BeforeEach(func() { namespace = "bar" })
						It("should not set the namespaced reflector as ready", func() { Expect(nsrfl.Ready()).To(BeFalse()) })
					})
				})

				Context("a namespaced reflector is retrieved", func() {
					var (
						namespace string
						retrieved manager.NamespacedReflector
						found     bool
					)

					JustBeforeEach(func() { retrieved, found = rfl.(*reflector).namespace(namespace) })
					When("the namespace exists", func() {
						BeforeEach(func() { namespace = localNamespace })
						It("should return a non-nil reflector", func() { Expect(retrieved).ToNot(BeNil()) })
						It("should return the reflector has been found", func() { Expect(found).To(BeTrue()) })
					})
					When("the namespace does not exist", func() {
						BeforeEach(func() { namespace = "baz" })
						It("should return a nil reflector", func() { Expect(retrieved).To(BeNil()) })
						It("should return the reflector has not been found", func() { Expect(found).To(BeFalse()) })
					})
				})

				Context("an item is handled", func() {
					var (
						ctx   context.Context
						key   types.NamespacedName
						ready bool
						err   error
					)

					BeforeEach(func() { ctx = context.Background() })
					JustBeforeEach(func() {
						if ready {
							nsrfl.SetReady()
						}
						err = rfl.(*reflector).handle(ctx, key)
					})
					When("the namespace exists", func() {
						BeforeEach(func() { key = types.NamespacedName{Namespace: localNamespace, Name: "foo"} })
						When("the namespaced reflector is not ready", func() {
							It("should return an error", func() { Expect(err).To(HaveOccurred()) })
							It("should not execute the namespaced reflector handler", func() { Expect(nsrfl.Handled).To(BeNumerically("==", 0)) })
						})
						When("the namespaced reflector is ready", func() {
							BeforeEach(func() { ready = true })
							It("should not return an error", func() { Expect(err).ToNot(HaveOccurred()) })
							It("should execute the namespaced reflector handler", func() { Expect(nsrfl.Handled).To(BeNumerically("==", 1)) })
						})
					})
					When("the namespace does not exist", func() {
						BeforeEach(func() { key = types.NamespacedName{Namespace: "whatever", Name: "foo"} })
						It("should not return an error", func() { Expect(err).ToNot(HaveOccurred()) })
						It("should not execute the namespaced reflector handler", func() { Expect(nsrfl.Handled).To(BeNumerically("==", 0)) })
					})
				})
			})

			Context("the handlers are generated", func() {
				var (
					handlers cache.ResourceEventHandler
					obj      corev1.Service
					expected types.NamespacedName
				)

				keyer := func(metadata metav1.Object) types.NamespacedName {
					const prefix = "Test"
					return types.NamespacedName{
						Namespace: prefix + metadata.GetNamespace(),
						Name:      prefix + metadata.GetName(),
					}
				}

				body := func() func() {
					return func() {
						Expect(rfl.(*reflector).workqueue.Len()).To(BeNumerically("==", 1))
						key, _ := rfl.(*reflector).workqueue.Get()
						Expect(key).To(Equal(expected))
					}
				}

				BeforeEach(func() {
					obj = corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "Name", Namespace: "Namespace"}}
					expected = types.NamespacedName{Name: "TestName", Namespace: "TestNamespace"}
				})
				JustBeforeEach(func() { handlers = rfl.(*reflector).handlers(keyer) })

				When("the AddFunc is executed", func() {
					JustBeforeEach(func() { handlers.OnAdd(&obj) })
					It("should return the correct namespaced name", body())
				})
				When("the UpdateFunc is executed", func() {
					JustBeforeEach(func() { handlers.OnUpdate(&obj, &obj) })
					It("should return the correct namespaced name", body())
				})
				When("the DeleteFunc is executed", func() {
					JustBeforeEach(func() { handlers.OnDelete(&obj) })
					It("should return the correct namespaced name", body())
				})
			})
		})
	})

	Describe("the NamespacedKeyer function", func() {
		const (
			name      = "name"
			namespace = "namespace"
		)

		var (
			input  metav1.Object
			output types.NamespacedName
		)

		BeforeEach(func() { input = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "whatever"}} })
		JustBeforeEach(func() { output = NamespacedKeyer(namespace)(input) })
		It("should return the same name of the object", func() { Expect(output.Name).To(BeIdenticalTo(name)) })
		It("should return the namespace given as parameter", func() { Expect(output.Namespace).To(BeIdenticalTo(namespace)) })
	})
})
