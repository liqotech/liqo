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

package generic

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
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
			fbrfl *reflectionfake.FallbackReflector

			workers uint

			NewFakeNamespacedReflector NamespacedReflectorFactoryFunc
			NewFakeFallbackReflector   FallbackReflectorFactoryFunc
		)

		BeforeEach(func() {
			NewFakeNamespacedReflector = func(opts *options.NamespacedOpts) manager.NamespacedReflector {
				nsrfl = reflectionfake.NewNamespacedReflector(opts)
				return nsrfl
			}

			NewFakeFallbackReflector = func(opts *options.ReflectorOpts) manager.FallbackReflector {
				fbrfl = reflectionfake.NewFallbackReflector(opts)
				return fbrfl
			}
		})

		Context("a new reflector is created", func() {
			JustBeforeEach(func() {
				rfl = NewReflector(reflectorName, NewFakeNamespacedReflector, NewFakeFallbackReflector,
					workers, offloadingv1beta1.CustomLiqo, ConcurrencyModeAll)
			})

			When("no workers are specified", func() {
				BeforeEach(func() { workers = 0 })
				It("should return a dummy reflector", func() { Expect(rfl).To(PointTo(BeAssignableToTypeOf(dummyreflector{}))) })
			})

			When("at least one worker is specified", func() {
				BeforeEach(func() { workers = 1 })
				It("should return a real reflector", func() { Expect(rfl).To(PointTo(BeAssignableToTypeOf(reflector{}))) })
			})
		})

		Context("a new real reflector is created", func() {
			BeforeEach(func() { workers = 10 })
			JustBeforeEach(func() {
				// Here, we use the internal function, to retrieve the real reflector also in case no workers are set.
				rfl = newReflector(reflectorName, NewFakeNamespacedReflector, NewFakeFallbackReflector,
					workers, offloadingv1beta1.CustomLiqo, ConcurrencyModeAll)
			})
			It("should return a non nil reflector", func() { Expect(rfl).ToNot(BeNil()) })
			It("should correctly populate the reflector fields", func() {
				Expect(rfl.(*reflector).name).To(BeIdenticalTo(reflectorName))
				Expect(rfl.(*reflector).workers).To(BeNumerically("==", 10))

				Expect(rfl.(*reflector).workqueue).ToNot(BeNil())

				Expect(rfl.(*reflector).reflectors).ToNot(BeNil())
				Expect(rfl.(*reflector).fallback).To(BeNil())
				Expect(rfl.(*reflector).namespacedFactory).ToNot(BeNil())
				Expect(rfl.(*reflector).fallbackFactory).ToNot(BeNil())
				Expect(rfl.(*reflector).reflectionType).ToNot(BeNil())
				Expect(rfl.(*reflector).concurrencyMode).ToNot(BeNil())
			})

			Context("the reflector is started", func() {
				var (
					opts   options.ReflectorOpts
					ctx    context.Context
					cancel context.CancelFunc
				)

				BeforeEach(func() {
					ctx, cancel = context.WithCancel(context.Background())
					workers = 0 /* do not start any child go routine */
					opts = options.ReflectorOpts{LocalClient: fake.NewSimpleClientset()}
				})
				JustBeforeEach(func() { rfl.Start(ctx, &opts) })
				AfterEach(func() { cancel() })

				It("should create a new fallback reflector", func() { Expect(rfl.(*reflector).fallback).To(Equal(fbrfl)) })
				It("should correctly propagate the reflector options", func() {
					Expect(fbrfl.Opts.LocalClient).To(Equal(opts.LocalClient))
					Expect(fbrfl.Opts.HandlerFactory).ToNot(BeNil())
				})

				Context("a new namespace is started", func() {
					var opts options.NamespacedOpts

					BeforeEach(func() {
						opts = options.NamespacedOpts{LocalNamespace: localNamespace, RemoteNamespace: remoteNamespace}
					})
					JustBeforeEach(func() { rfl.StartNamespace(&opts) })

					It("should create a new namespaced reflector", func() {
						Expect(rfl.(*reflector).reflectors).To(HaveKeyWithValue(localNamespace, nsrfl))
					})
					It("should correctly propagate the reflector options", func() {
						Expect(nsrfl.Opts.LocalNamespace).To(Equal(localNamespace))
						Expect(nsrfl.Opts.RemoteNamespace).To(Equal(remoteNamespace))
						Expect(opts.HandlerFactory).ToNot(BeNil())
					})

					When("the fallback handler is set", func() {
						It("should enqueue the returned elements", func() {
							Expect(rfl.(*reflector).workqueue.Len()).To(BeNumerically("==", 1))
							key, _ := rfl.(*reflector).workqueue.Get()
							Expect(key).To(Equal(types.NamespacedName{Namespace: localNamespace, Name: remoteNamespace}))
						})
					})

					Context("the same namespace is stopped", func() {
						JustBeforeEach(func() { rfl.StopNamespace(localNamespace, remoteNamespace) })
						It("should remove the namespaced reflector", func() {
							Expect(rfl.(*reflector).reflectors).ToNot(HaveKeyWithValue(localNamespace, nsrfl))
						})

						When("the fallback handler is set", func() {
							It("should enqueue the returned elements", func() {
								Expect(rfl.(*reflector).workqueue.Len()).To(BeNumerically("==", 1))
								key, _ := rfl.(*reflector).workqueue.Get()
								Expect(key).To(Equal(types.NamespacedName{Namespace: localNamespace, Name: remoteNamespace}))
							})
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
							key   types.NamespacedName
							ready bool
							err   error
						)

						BeforeEach(func() { ready = false })
						JustBeforeEach(func() {
							if ready {
								nsrfl.SetReady()
								fbrfl.SetReady()
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
							When("the fallback handler is not set", func() {
								BeforeEach(func() { NewFakeFallbackReflector = WithoutFallback() })
								It("should not return an error", func() { Expect(err).ToNot(HaveOccurred()) })
								It("should not execute the namespaced reflector handler", func() { Expect(nsrfl.Handled).To(BeNumerically("==", 0)) })
							})
							When("the fallback handler is set and it is ready", func() {
								BeforeEach(func() { ready = true })
								It("should not return an error", func() { Expect(err).ToNot(HaveOccurred()) })
								It("should not execute the namespaced reflector handler", func() { Expect(nsrfl.Handled).To(BeNumerically("==", 0)) })
								It("should execute the fallback handler", func() { Expect(fbrfl.Handled).To(BeNumerically("==", 1)) })
							})
							When("the fallback handler is set and it not ready", func() {
								It("should return an error", func() { Expect(err).To(HaveOccurred()) })
								It("should not execute the namespaced reflector handler", func() { Expect(nsrfl.Handled).To(BeNumerically("==", 0)) })
								It("should not execute the fallback handler", func() { Expect(fbrfl.Handled).To(BeNumerically("==", 0)) })
							})
						})
					})
				})
			})

			Context("the handlers are generated", func() {
				var (
					handlers cache.ResourceEventHandler
					obj      corev1.Service
					filters  []options.EventFilter
				)

				keyer := func(metadata metav1.Object) []types.NamespacedName {
					const prefix = "Test"
					return []types.NamespacedName{{
						Namespace: prefix + metadata.GetNamespace() + "1",
						Name:      prefix + metadata.GetName() + "1",
					}, {
						Namespace: prefix + metadata.GetNamespace() + "2",
						Name:      prefix + metadata.GetName() + "2",
					}}
				}

				body := func() func() {
					return func() {
						Expect(rfl.(*reflector).workqueue.Len()).To(BeNumerically("==", 2))
						key, _ := rfl.(*reflector).workqueue.Get()
						Expect(key).To(Equal(types.NamespacedName{Name: "TestName1", Namespace: "TestNamespace1"}))
						key, _ = rfl.(*reflector).workqueue.Get()
						Expect(key).To(Equal(types.NamespacedName{Name: "TestName2", Namespace: "TestNamespace2"}))
					}
				}

				bodyEmpty := func() func() {
					return func() {
						Expect(rfl.(*reflector).workqueue.Len()).To(BeNumerically("==", 0))
					}
				}

				BeforeEach(func() {
					obj = corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "Name", Namespace: "Namespace"}}
					filters = nil
				})
				JustBeforeEach(func() { handlers = rfl.(*reflector).handlers(keyer, filters...) })

				When("the AddFunc is executed", func() {
					JustBeforeEach(func() { handlers.OnAdd(&obj, false) })

					When("no event filter is specified", func() {
						It("should return the correct namespaced name", body())
					})
					When("the Create filter is specified", func() {
						BeforeEach(func() { filters = []options.EventFilter{options.EventFilterCreate} })
						It("should enqueue no elements", bodyEmpty())
					})
					When("the other filters are specified", func() {
						BeforeEach(func() { filters = []options.EventFilter{options.EventFilterUpdate, options.EventFilterDelete} })
						It("should return the correct namespaced name", body())
					})
				})
				When("the UpdateFunc is executed", func() {
					JustBeforeEach(func() { handlers.OnUpdate(&obj, &obj) })

					When("no event filter is specified", func() {
						It("should return the correct namespaced name", body())
					})
					When("the Update filter is specified", func() {
						BeforeEach(func() { filters = []options.EventFilter{options.EventFilterUpdate} })
						It("should enqueue no elements", bodyEmpty())
					})
					When("the other filters are specified", func() {
						BeforeEach(func() { filters = []options.EventFilter{options.EventFilterCreate, options.EventFilterDelete} })
						It("should return the correct namespaced name", body())
					})
				})
				When("the DeleteFunc is executed", func() {
					JustBeforeEach(func() { handlers.OnDelete(&obj) })

					When("no event filter is specified", func() {
						It("should return the correct namespaced name", body())
					})
					When("the Delete filter is specified", func() {
						BeforeEach(func() { filters = []options.EventFilter{options.EventFilterDelete} })
						It("should enqueue no elements", bodyEmpty())
					})
					When("the other filters are specified", func() {
						BeforeEach(func() { filters = []options.EventFilter{options.EventFilterCreate, options.EventFilterUpdate} })
						It("should return the correct namespaced name", body())
					})
				})
			})
		})
	})

	Describe("the *Keyer functions", func() {
		const (
			name      = "name"
			namespace = "namespace"
		)

		var (
			input metav1.Object
			keyer options.Keyer
		)

		Describe("the BasicKeyer function", func() {
			BeforeEach(func() { input = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}} })
			JustBeforeEach(func() { keyer = BasicKeyer() })
			It("should return a single object", func() { Expect(keyer(input)).To(HaveLen(1)) })
			It("should return the same name of the object", func() { Expect(keyer(input)[0].Name).To(BeIdenticalTo(name)) })
			It("should return the same namespace of the object", func() { Expect(keyer(input)[0].Namespace).To(BeIdenticalTo(namespace)) })
		})

		Describe("the NamespacedKeyer function", func() {
			BeforeEach(func() { input = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "whatever"}} })
			JustBeforeEach(func() { keyer = NamespacedKeyer(namespace) })
			It("should return a single object", func() { Expect(keyer(input)).To(HaveLen(1)) })
			It("should return the same name of the object", func() { Expect(keyer(input)[0].Name).To(BeIdenticalTo(name)) })
			It("should return the namespace given as parameter", func() { Expect(keyer(input)[0].Namespace).To(BeIdenticalTo(namespace)) })
		})
	})
})
