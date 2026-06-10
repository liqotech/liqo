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

package dra_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/api/resource/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
	resourcev1listers "k8s.io/client-go/listers/resource/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"

	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/dra"
)

// errResourceSliceLister is a stub ResourceSliceLister whose Get always returns
// the configured error. Used to test error propagation paths that envtest can't
// reproduce via reactors (informer-backed listers swallow API errors).
type errResourceSliceLister struct {
	err error
}

func (e *errResourceSliceLister) List(_ labels.Selector) ([]*resourcev1.ResourceSlice, error) {
	return nil, e.err
}
func (e *errResourceSliceLister) Get(_ string) (*resourcev1.ResourceSlice, error) {
	return nil, e.err
}

func newSliceIndexer() cache.Indexer {
	return cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
}

func newNodeLister(nodes ...*corev1.Node) corev1listers.NodeLister {
	idx := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, n := range nodes {
		Expect(idx.Add(n)).To(Succeed())
	}
	return corev1listers.NewNodeLister(idx)
}

var _ = Describe("ResourceSliceReflector", func() {
	const sliceName = "slice-1"

	var (
		reflector         *dra.ResourceSliceReflector
		localSlicesIndex  cache.Indexer
		remoteSlicesIndex cache.Indexer
		localNodes        corev1listers.NodeLister
		opts              *forge.ForgingOpts
	)

	// localNode mirrors the suite-created virtual node so OwnerReference
	// assertions can compare against LiqoNodeUID.
	mkLocalNode := func() *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: LiqoNodeName, UID: LiqoNodeUID},
		}
	}

	BeforeEach(func() {
		opts = testutil.FakeForgingOpts()
		localSlicesIndex = newSliceIndexer()
		remoteSlicesIndex = newSliceIndexer()
		localNodes = newNodeLister(mkLocalNode())
	})

	JustBeforeEach(func() {
		reflector = dra.NewResourceSliceReflectorForTest(
			localClient, remoteClient,
			resourcev1listers.NewResourceSliceLister(localSlicesIndex),
			resourcev1listers.NewResourceSliceLister(remoteSlicesIndex),
			localNodes,
			opts,
		)
	})

	AfterEach(func() {
		// Best-effort cleanup of any slice the test may have created on the envtest API.
		_ = localClient.ResourceV1().ResourceSlices().Delete(ctx, sliceName, metav1.DeleteOptions{})
	})

	Describe("handle", func() {
		Context("when both local and remote are missing", func() {
			It("should be a no-op and return nil", func() {
				Expect(dra.Handle(reflector, ctx, sliceName)).To(Succeed())
				_, err := localClient.ResourceV1().ResourceSlices().Get(ctx, sliceName, metav1.GetOptions{})
				Expect(kerrors.IsNotFound(err)).To(BeTrue())
			})
		})

		Context("when the remote vanished", func() {
			When("the local slice exists and is reflected by Liqo", func() {
				BeforeEach(func() {
					created, err := localClient.ResourceV1().ResourceSlices().Create(ctx, &resourcev1.ResourceSlice{
						ObjectMeta: metav1.ObjectMeta{
							Name:   sliceName,
							Labels: forge.ReflectionLabels(),
						},
						Spec: resourcev1.ResourceSliceSpec{
							Driver:   fakeDriverName,
							Pool:     resourcev1.ResourcePool{Name: fakePoolName, ResourceSliceCount: 1},
							NodeName: ptr.To(LiqoNodeName),
						},
					}, metav1.CreateOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(localSlicesIndex.Add(created)).To(Succeed())
				})

				It("should delete the local slice", func() {
					Expect(dra.Handle(reflector, ctx, sliceName)).To(Succeed())
					_, err := localClient.ResourceV1().ResourceSlices().Get(ctx, sliceName, metav1.GetOptions{})
					Expect(kerrors.IsNotFound(err)).To(BeTrue())
				})
			})

			When("the local slice exists but is not reflected by Liqo", func() {
				BeforeEach(func() {
					created, err := localClient.ResourceV1().ResourceSlices().Create(ctx, &resourcev1.ResourceSlice{
						ObjectMeta: metav1.ObjectMeta{Name: sliceName},
						Spec: resourcev1.ResourceSliceSpec{
							Driver:   fakeDriverName,
							Pool:     resourcev1.ResourcePool{Name: fakePoolName, ResourceSliceCount: 1},
							NodeName: ptr.To(LiqoNodeName),
						},
					}, metav1.CreateOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(localSlicesIndex.Add(created)).To(Succeed())
				})

				It("should not delete the local slice", func() {
					Expect(dra.Handle(reflector, ctx, sliceName)).To(Succeed())
					_, err := localClient.ResourceV1().ResourceSlices().Get(ctx, sliceName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when the remote slice exists", func() {
			var remote *resourcev1.ResourceSlice

			BeforeEach(func() {
				remote = &resourcev1.ResourceSlice{
					ObjectMeta: metav1.ObjectMeta{
						Name: sliceName,
						Labels: map[string]string{
							"foo":                             barVal,
							testutil.FakeNotReflectedLabelKey: trueVal,
						},
					},
					Spec: resourcev1.ResourceSliceSpec{
						Driver:   fakeDriverName,
						Pool:     resourcev1.ResourcePool{Name: fakePoolName, ResourceSliceCount: 1},
						NodeName: ptr.To(LiqoNodeName),
						Devices:  []resourcev1.Device{{Name: "dev-1"}},
					},
				}
			})

			JustBeforeEach(func() { Expect(remoteSlicesIndex.Add(remote)).To(Succeed()) })

			When("the remote has no NodeName", func() {
				BeforeEach(func() { remote.Spec.NodeName = nil })
				It("should be a no-op", func() {
					Expect(dra.Handle(reflector, ctx, sliceName)).To(Succeed())
					_, err := localClient.ResourceV1().ResourceSlices().Get(ctx, sliceName, metav1.GetOptions{})
					Expect(kerrors.IsNotFound(err)).To(BeTrue())
				})
			})

			When("the remote has an empty-string NodeName", func() {
				BeforeEach(func() { remote.Spec.NodeName = ptr.To("") })
				It("should be a no-op", func() {
					Expect(dra.Handle(reflector, ctx, sliceName)).To(Succeed())
					_, err := localClient.ResourceV1().ResourceSlices().Get(ctx, sliceName, metav1.GetOptions{})
					Expect(kerrors.IsNotFound(err)).To(BeTrue())
				})
			})

			When("the local virtual node is not in cache", func() {
				BeforeEach(func() {
					localNodes = newNodeLister()
				})
				It("should return nil and not create the local slice", func() {
					Expect(dra.Handle(reflector, ctx, sliceName)).To(Succeed())
					_, err := localClient.ResourceV1().ResourceSlices().Get(ctx, sliceName, metav1.GetOptions{})
					Expect(kerrors.IsNotFound(err)).To(BeTrue())
				})
			})

			When("the local slice does not yet exist", func() {
				It("should create the local slice with reflection labels and OwnerReference to the local node", func() {
					Expect(dra.Handle(reflector, ctx, sliceName)).To(Succeed())

					got, err := localClient.ResourceV1().ResourceSlices().Get(ctx, sliceName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(got.Labels).To(HaveKeyWithValue(fooVal, barVal))
					Expect(got.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
					Expect(got.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(got.OwnerReferences).To(HaveLen(1))
					Expect(got.OwnerReferences[0].Name).To(Equal(LiqoNodeName))
					Expect(got.OwnerReferences[0].UID).To(Equal(LiqoNodeUID))
					Expect(got.Spec.Driver).To(Equal(fakeDriverName))
				})
			})

			When("the local slice exists, is reflected, and is already up-to-date", func() {
				var rvBefore string

				BeforeEach(func() {
					desired := forge.LocalResourceSlice(remote, mkLocalNode(),
						opts.LabelsNotReflected, opts.AnnotationsNotReflected)
					created, err := localClient.ResourceV1().ResourceSlices().Create(ctx, desired, metav1.CreateOptions{})
					Expect(err).ToNot(HaveOccurred())
					rvBefore = created.ResourceVersion
					Expect(localSlicesIndex.Add(created)).To(Succeed())
				})

				It("should not issue an Update (resourceVersion unchanged)", func() {
					Expect(dra.Handle(reflector, ctx, sliceName)).To(Succeed())
					got, err := localClient.ResourceV1().ResourceSlices().Get(ctx, sliceName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(got.ResourceVersion).To(Equal(rvBefore))
				})
			})

			When("the local slice exists, is reflected, but has stale labels", func() {
				BeforeEach(func() {
					stale := forge.LocalResourceSlice(remote, mkLocalNode(),
						opts.LabelsNotReflected, opts.AnnotationsNotReflected)
					stale.Labels["stale"] = "yes"
					created, err := localClient.ResourceV1().ResourceSlices().Create(ctx, stale, metav1.CreateOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(localSlicesIndex.Add(created)).To(Succeed())
				})

				It("should issue an Update that reconciles labels to match the desired state", func() {
					Expect(dra.Handle(reflector, ctx, sliceName)).To(Succeed())
					got, err := localClient.ResourceV1().ResourceSlices().Get(ctx, sliceName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(got.Labels).ToNot(HaveKey("stale"))
					Expect(got.Labels).To(HaveKeyWithValue(fooVal, barVal))
					Expect(got.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
				})
			})

			When("the local slice exists but is not reflected by Liqo", func() {
				var rvBefore string

				BeforeEach(func() {
					created, err := localClient.ResourceV1().ResourceSlices().Create(ctx, &resourcev1.ResourceSlice{
						ObjectMeta: metav1.ObjectMeta{Name: sliceName},
						Spec: resourcev1.ResourceSliceSpec{
							Driver:   "other",
							Pool:     resourcev1.ResourcePool{Name: fakePoolName, ResourceSliceCount: 1},
							NodeName: ptr.To(LiqoNodeName),
						},
					}, metav1.CreateOptions{})
					Expect(err).ToNot(HaveOccurred())
					rvBefore = created.ResourceVersion
					Expect(localSlicesIndex.Add(created)).To(Succeed())
				})

				It("should skip without modifying the local slice", func() {
					Expect(dra.Handle(reflector, ctx, sliceName)).To(Succeed())
					got, err := localClient.ResourceV1().ResourceSlices().Get(ctx, sliceName, metav1.GetOptions{})
					Expect(err).ToNot(HaveOccurred())
					Expect(got.ResourceVersion).To(Equal(rvBefore))
				})
			})
		})

		Context("when the local lister returns a non-NotFound error", func() {
			JustBeforeEach(func() {
				reflector = dra.NewResourceSliceReflectorForTest(
					localClient, remoteClient,
					&errResourceSliceLister{err: errors.New("local boom")},
					resourcev1listers.NewResourceSliceLister(remoteSlicesIndex),
					localNodes,
					opts,
				)
			})

			It("should propagate the error", func() {
				err := dra.Handle(reflector, ctx, sliceName)
				Expect(err).To(MatchError(ContainSubstring("local boom")))
			})
		})

		Context("when the remote lister returns a non-NotFound error", func() {
			JustBeforeEach(func() {
				reflector = dra.NewResourceSliceReflectorForTest(
					localClient, remoteClient,
					resourcev1listers.NewResourceSliceLister(localSlicesIndex),
					&errResourceSliceLister{err: errors.New("remote boom")},
					localNodes,
					opts,
				)
			})

			It("should propagate the error", func() {
				err := dra.Handle(reflector, ctx, sliceName)
				Expect(err).To(MatchError(ContainSubstring("remote boom")))
			})
		})
	})

	Describe("enqueueRemoteSlicesForNode", func() {
		mkSlice := func(name, nodeName string) *resourcev1.ResourceSlice {
			s := &resourcev1.ResourceSlice{ObjectMeta: metav1.ObjectMeta{Name: name}}
			if nodeName != "" {
				s.Spec.NodeName = ptr.To(nodeName)
			}
			return s
		}

		It("should only enqueue slices whose NodeName matches", func() {
			Expect(remoteSlicesIndex.Add(mkSlice("a", LiqoNodeName))).To(Succeed())
			Expect(remoteSlicesIndex.Add(mkSlice("b", "other-node"))).To(Succeed())
			Expect(remoteSlicesIndex.Add(mkSlice("c", LiqoNodeName))).To(Succeed())

			before := reflector.WorkqueueLen()
			dra.EnqueueRemoteSlicesForNode(reflector, LiqoNodeName)
			added := reflector.WorkqueueLen() - before
			Expect(added).To(Equal(2))
		})

		It("should skip slices with nil NodeName", func() {
			Expect(remoteSlicesIndex.Add(mkSlice("a", ""))).To(Succeed())

			before := reflector.WorkqueueLen()
			dra.EnqueueRemoteSlicesForNode(reflector, LiqoNodeName)
			Expect(reflector.WorkqueueLen() - before).To(Equal(0))
		})

		It("should not panic on an empty cache", func() {
			Expect(func() { dra.EnqueueRemoteSlicesForNode(reflector, LiqoNodeName) }).ToNot(Panic())
		})
	})

	Describe("Resync", func() {
		It("should add nothing to the workqueue when there are no slices", func() {
			before := reflector.WorkqueueLen()
			Expect(reflector.Resync()).To(Succeed())
			Expect(reflector.WorkqueueLen() - before).To(Equal(0))
		})

		It("should enqueue every remote slice when only remotes exist", func() {
			Expect(remoteSlicesIndex.Add(&resourcev1.ResourceSlice{ObjectMeta: metav1.ObjectMeta{Name: "a"}})).To(Succeed())
			Expect(remoteSlicesIndex.Add(&resourcev1.ResourceSlice{ObjectMeta: metav1.ObjectMeta{Name: "b"}})).To(Succeed())

			before := reflector.WorkqueueLen()
			Expect(reflector.Resync()).To(Succeed())
			Expect(reflector.WorkqueueLen() - before).To(Equal(2))
		})

		It("should enqueue every local slice when only locals exist", func() {
			Expect(localSlicesIndex.Add(&resourcev1.ResourceSlice{ObjectMeta: metav1.ObjectMeta{Name: "a"}})).To(Succeed())
			Expect(localSlicesIndex.Add(&resourcev1.ResourceSlice{ObjectMeta: metav1.ObjectMeta{Name: "b"}})).To(Succeed())

			before := reflector.WorkqueueLen()
			Expect(reflector.Resync()).To(Succeed())
			Expect(reflector.WorkqueueLen() - before).To(Equal(2))
		})
	})
})
