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
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("Handler tests", func() {

	const (
		localNamespace  = "foo"
		remoteNamespace = "bar"
	)

	var (
		ctx    context.Context
		cancel context.CancelFunc

		localCluster  liqov1beta1.ClusterID
		remoteCluster liqov1beta1.ClusterID

		gvr       schema.GroupVersionResource
		ownership consts.OwnershipType

		reflector                 Reflector
		local, remote             dynamic.Interface
		localBefore, remoteBefore offloadingv1beta1.NamespaceMap

		key item
		err error
	)

	Item := func(name string) item { return item{gvr: gvr, name: name} }
	Lister := func(ctx context.Context, client dynamic.Interface, namespace string, gvr schema.GroupVersionResource) cache.GenericNamespaceLister {
		factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(client, 0, namespace, func(lo *metav1.ListOptions) {})
		informer := factory.ForResource(gvr)
		factory.Start(ctx.Done())
		factory.WaitForCacheSync(ctx.Done())
		return informer.Lister().ByNamespace(namespace)
	}

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		gvr = offloadingv1beta1.NamespaceMapGroupVersionResource
		ownership = consts.OwnershipLocal

		// Fill with fake data, to avoid issues if not overwritten later with real parameters
		localBefore = offloadingv1beta1.NamespaceMap{
			TypeMeta:   metav1.TypeMeta{APIVersion: offloadingv1beta1.SchemeGroupVersion.String(), Kind: "NamespaceMap"},
			ObjectMeta: metav1.ObjectMeta{Name: "not-existing", Namespace: "not-existing"}}
		remoteBefore = offloadingv1beta1.NamespaceMap{
			TypeMeta:   metav1.TypeMeta{APIVersion: offloadingv1beta1.SchemeGroupVersion.String(), Kind: "NamespaceMap"},
			ObjectMeta: metav1.ObjectMeta{Name: "not-existing", Namespace: "not-existing"}}

		localCluster = liqov1beta1.ClusterID("local-cluster-id")
		remoteCluster = liqov1beta1.ClusterID("remote-cluster-id")
	})

	AfterEach(func() { cancel() })

	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(ipamv1alpha1.AddToScheme(scheme))

		local = fake.NewSimpleDynamicClient(scheme, localBefore.DeepCopy())
		remote = fake.NewSimpleDynamicClient(scheme, remoteBefore.DeepCopy())

		reflector = Reflector{
			manager: &Manager{
				client: local,
			},

			remoteClient:    remote,
			localNamespace:  localNamespace,
			remoteNamespace: remoteNamespace,
			localClusterID:  localCluster,
			remoteClusterID: remoteCluster,

			resources: map[schema.GroupVersionResource]*reflectedResource{
				gvr: {
					gvr:       gvr,
					ownership: ownership,
					local:     Lister(ctx, local, localNamespace, gvr),
					remote:    Lister(ctx, remote, remoteNamespace, gvr),
				},
			},
		}

		err = reflector.handle(ctx, key)
	})

	When("the local object does not exist", func() {
		const name = "not-existing"
		BeforeEach(func() { key = Item(name) })

		WhenBody := func(createRemote bool) func() {
			return func() {
				BeforeEach(func() {
					if createRemote {
						remoteBefore.ObjectMeta = metav1.ObjectMeta{Name: name, Namespace: remoteNamespace}
					}
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("the remote object should not be created", func() {
					_, err = remote.Resource(gvr).Namespace(remoteNamespace).Get(ctx, name, metav1.GetOptions{})
					Expect(err).To(HaveOccurred())
					Expect(kerrors.IsNotFound(err)).To(BeTrue())
				})
			}
		}

		When("the remote object does not exist", WhenBody(false))
		When("the remote object does exist", WhenBody(true))
	})

	When("the local object is being deleted", func() {
		const name = "existing"
		var localAfter offloadingv1beta1.NamespaceMap

		BeforeEach(func() {
			localBefore.ObjectMeta = metav1.ObjectMeta{
				Name: name, Namespace: localNamespace,
				Labels: map[string]string{
					consts.ReplicationRequestedLabel:   strconv.FormatBool(true),
					consts.ReplicationDestinationLabel: string(reflector.remoteClusterID)},
				DeletionTimestamp: &metav1.Time{Time: time.Now()},
				Finalizers:        []string{finalizer},
			}
			key = Item(name)
		})

		JustBeforeEach(func() {
			// Retrieve the local object after the modifications
			unstr, err2 := local.Resource(gvr).Namespace(localNamespace).Get(ctx, name, metav1.GetOptions{})
			Expect(err2).ToNot(HaveOccurred())
			Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &localAfter)).To(Succeed())
		})

		WhenBody := func(createRemote bool) func() {
			return func() {
				BeforeEach(func() {
					if createRemote {
						remoteBefore.ObjectMeta = metav1.ObjectMeta{Name: name, Namespace: remoteNamespace}
					}
				})

				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should ensure the local object finalizer correctness", func() {
					if createRemote {
						// The finalizer is removed only when the remote object disappears.
						Expect(localAfter.Finalizers).To(ContainElement(finalizer))
					} else {
						Expect(localAfter.Finalizers).ToNot(ContainElement(finalizer))
					}
				})
				It("the remote object should not be created", func() {
					_, err = remote.Resource(gvr).Namespace(remoteNamespace).Get(ctx, name, metav1.GetOptions{})
					Expect(err).To(HaveOccurred())
					Expect(kerrors.IsNotFound(err)).To(BeTrue())
				})
			}
		}

		When("the remote object does not exist", WhenBody(false))
		When("the remote object does exist", WhenBody(true))
	})

	When("the local object does exist", func() {
		const name = "existing"
		var localAfter, remoteAfter offloadingv1beta1.NamespaceMap

		BeforeEach(func() {
			localBefore.ObjectMeta = metav1.ObjectMeta{
				Name: name, Namespace: localNamespace,
				Labels: map[string]string{
					consts.ReplicationRequestedLabel:   strconv.FormatBool(true),
					consts.ReplicationDestinationLabel: string(reflector.remoteClusterID),
					consts.LocalResourceOwnership:      "tester",
					"foo":                              "bar"},
			}
			localBefore.Spec = offloadingv1beta1.NamespaceMapSpec{DesiredMapping: map[string]string{"foo": "bar"}}
			localBefore.Status = offloadingv1beta1.NamespaceMapStatus{
				CurrentMapping: map[string]offloadingv1beta1.RemoteNamespaceStatus{"foo": {RemoteNamespace: "bar"}}}
			key = Item(name)
		})

		JustBeforeEach(func() {
			// Retrieve the local object after the modifications
			unstr, err2 := local.Resource(gvr).Namespace(localNamespace).Get(ctx, name, metav1.GetOptions{})
			Expect(err2).ToNot(HaveOccurred())
			Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &localAfter)).To(Succeed())

			// Retrieve the remote object after the modifications
			unstr, err2 = remote.Resource(gvr).Namespace(remoteNamespace).Get(ctx, name, metav1.GetOptions{})
			Expect(err2).ToNot(HaveOccurred())
			Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &remoteAfter)).To(Succeed())
		})

		StatusBody := func() func() {
			return func() {
				Context("Local ownership", func() {
					It("the local status should have been correctly replicated to the remote object", func() {
						Expect(localAfter.Status).To(Equal(localBefore.Status))
						Expect(remoteAfter.Status).To(Equal(localBefore.Status))
					})
				})
				Context("Shared ownership", func() {
					BeforeEach(func() { ownership = consts.OwnershipShared })
					It("the remote status should have been correctly replicated to the local object", func() {
						Expect(remoteAfter.Status).To(Equal(remoteBefore.Status))
						Expect(localAfter.Status).To(Equal(remoteBefore.Status))
					})
				})
			}
		}

		When("the remote object does not exist", func() {
			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should add a finalizer to the local object", func() {
				Expect(localAfter.Finalizers).To(ContainElement(finalizer))
			})

			It("the labels should have been correctly replicated to the remote object", func() {
				Expect(localAfter.Labels).To(Equal(localBefore.Labels))
				Expect(remoteAfter.Labels).To(HaveKeyWithValue(consts.ReplicationRequestedLabel, strconv.FormatBool(false)))
				Expect(remoteAfter.Labels).To(HaveKeyWithValue(consts.ReplicationDestinationLabel, string(reflector.remoteClusterID)))
				Expect(remoteAfter.Labels).To(HaveKeyWithValue(consts.ReplicationOriginLabel, string(localCluster)))
				Expect(remoteAfter.Labels).To(HaveKeyWithValue(consts.ReplicationStatusLabel, strconv.FormatBool(true)))
				Expect(remoteAfter.Labels).NotTo(HaveKey(consts.LocalResourceOwnership))
				Expect(remoteAfter.Labels).To(HaveKeyWithValue("foo", "bar"))
			})
			It("the annotations should have been correctly replicated to the remote object", func() {
				Expect(localAfter.Annotations).To(Equal(localBefore.Annotations))
				Expect(remoteAfter.Annotations).To(Equal(localBefore.Annotations))
			})
			It("the spec should have been correctly replicated to the remote object", func() {
				Expect(localAfter.Spec).To(Equal(localBefore.Spec))
				Expect(remoteAfter.Spec).To(Equal(localBefore.Spec))
			})

			Describe("status replication", StatusBody())
		})

		When("the remote object already exists", func() {
			BeforeEach(func() {
				remoteBefore.ObjectMeta = metav1.ObjectMeta{Name: name, Namespace: remoteNamespace}
				localBefore.Spec = offloadingv1beta1.NamespaceMapSpec{
					DesiredMapping: map[string]string{"something": "wrong"},
				}
				localBefore.Status = offloadingv1beta1.NamespaceMapStatus{
					CurrentMapping: map[string]offloadingv1beta1.RemoteNamespaceStatus{"something": {RemoteNamespace: "wrong"}}}
			})

			It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			It("should add a finalizer to the local object", func() {
				Expect(localAfter.Finalizers).To(ContainElement(finalizer))
			})

			It("the spec should have been correctly replicated to the remote object", func() {
				Expect(localAfter.Spec).To(Equal(localBefore.Spec))
				Expect(remoteAfter.Spec).To(Equal(localBefore.Spec))
			})

			Describe("status replication", StatusBody())
		})
	})
})
