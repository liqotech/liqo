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

package custom_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/trace"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/custom"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	LocalNamespace  = "local-namespace"
	RemoteNamespace = "remote-namespace"

	LocalClusterID  = "local-cluster-id"
	RemoteClusterID = "remote-cluster-id"

	LiqoNodeName = "local-node"
	LiqoNodeIP   = "1.1.1.1"

	WidgetName = "test-widget"
)

var (
	gvr = schema.GroupVersionResource{Group: "example.io", Version: "v1", Resource: "widgets"}
	gvk = schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Widget"}

	ctx    context.Context
	cancel context.CancelFunc

	localClient, remoteClient dynamic.Interface
)

func TestCustom(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Custom Resource Reflection Suite")
}

var _ = BeforeSuite(func() {
	LogsToGinkgoWriter()
	forge.Init(LocalClusterID, RemoteClusterID, LiqoNodeName, LiqoNodeIP)
})

var _ = BeforeEach(func() {
	ctx, cancel = context.WithCancel(context.Background())

	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		gvr: "WidgetList",
	}
	localClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	remoteClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
})

var _ = AfterEach(func() { cancel() })

func newWidget(name, namespace string, labels, annotations map[string]string, spec, status map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetLabels(labels)
	obj.SetAnnotations(annotations)
	if spec != nil {
		_ = unstructured.SetNestedMap(obj.Object, spec, "spec")
	}
	if status != nil {
		_ = unstructured.SetNestedMap(obj.Object, status, "status")
	}
	return obj
}

var FakeEventHandler = func(options.Keyer, ...options.EventFilter) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ interface{}) {},
		UpdateFunc: func(_, _ interface{}) {},
		DeleteFunc: func(_ interface{}) {},
	}
}

var _ = Describe("GVR Reflection", func() {
	Describe("NewGVRReflector", func() {
		It("should create a non-nil reflector", func() {
			cfg := &offloadingv1beta1.ReflectorConfig{NumWorkers: 1, Type: offloadingv1beta1.AllowList}
			Expect(custom.NewGVRReflector(gvr, cfg)).NotTo(BeNil())
		})

		It("should default to AllowList when type is empty", func() {
			cfg := &offloadingv1beta1.ReflectorConfig{NumWorkers: 1}
			Expect(custom.NewGVRReflector(gvr, cfg)).NotTo(BeNil())
		})
	})

	Describe("Handle", func() {
		var (
			reflector      manager.NamespacedReflector
			reflectionType offloadingv1beta1.ReflectionType
			err            error
		)

		GetRemote := func(name string) *unstructured.Unstructured {
			obj, getErr := remoteClient.Resource(gvr).Namespace(RemoteNamespace).Get(ctx, name, metav1.GetOptions{})
			Expect(getErr).ToNot(HaveOccurred())
			return obj
		}

		GetLocal := func(name string) *unstructured.Unstructured {
			obj, getErr := localClient.Resource(gvr).Namespace(LocalNamespace).Get(ctx, name, metav1.GetOptions{})
			Expect(getErr).ToNot(HaveOccurred())
			return obj
		}

		CreateLocal := func(obj *unstructured.Unstructured) *unstructured.Unstructured {
			created, createErr := localClient.Resource(gvr).Namespace(LocalNamespace).Create(ctx, obj, metav1.CreateOptions{})
			Expect(createErr).ToNot(HaveOccurred())
			return created
		}

		CreateRemote := func(obj *unstructured.Unstructured) *unstructured.Unstructured {
			created, createErr := remoteClient.Resource(gvr).Namespace(RemoteNamespace).Create(ctx, obj, metav1.CreateOptions{})
			Expect(createErr).ToNot(HaveOccurred())
			return created
		}

		BeforeEach(func() {
			reflectionType = offloadingv1beta1.AllowList
		})

		JustBeforeEach(func() {
			localFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(localClient, 0, LocalNamespace, nil)
			remoteFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(remoteClient, 0, RemoteNamespace, nil)

			reflector = custom.NewNamespacedGVRReflector(gvr)(options.NewNamespaced().
				WithLocal(LocalNamespace, nil, nil).
				WithRemote(RemoteNamespace, nil, nil).
				WithDynamicLocal(localClient, localFactory).
				WithDynamicRemote(remoteClient, remoteFactory).
				WithHandlerFactory(FakeEventHandler).
				WithEventBroadcaster(record.NewBroadcaster()).
				WithReflectionType(reflectionType).
				WithForgingOpts(FakeForgingOpts()))

			localFactory.Start(ctx.Done())
			remoteFactory.Start(ctx.Done())
			localFactory.WaitForCacheSync(ctx.Done())
			remoteFactory.WaitForCacheSync(ctx.Done())

			err = reflector.Handle(trace.ContextWithTrace(ctx, trace.New("CustomResource")), WidgetName)
		})

		When("the local object does not exist", func() {
			When("the remote object does not exist", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
			})

			When("the remote object exists and is reflected", func() {
				BeforeEach(func() {
					CreateRemote(newWidget(WidgetName, RemoteNamespace, forge.ReflectionLabels(), nil,
						map[string]interface{}{"value": "v1"}, nil))
				})

				It("should succeed and delete the remote object", func() {
					Expect(err).ToNot(HaveOccurred())
					_, getErr := remoteClient.Resource(gvr).Namespace(RemoteNamespace).Get(ctx, WidgetName, metav1.GetOptions{})
					Expect(getErr).To(BeNotFound())
				})
			})
		})

		When("the local object exists with allow annotation", func() {
			BeforeEach(func() {
				CreateLocal(newWidget(WidgetName, LocalNamespace,
					map[string]string{"app": "demo", FakeNotReflectedLabelKey: "true"},
					map[string]string{consts.AllowReflectionAnnotationKey: "true", "anno": "val", FakeNotReflectedAnnotKey: "true"},
					map[string]interface{}{"replicas": int64(3)},
					nil))
			})

			When("the remote object does not exist", func() {
				It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })

				It("should create the remote twin with local spec and reflection labels", func() {
					remote := GetRemote(WidgetName)
					Expect(remote.GetLabels()).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
					Expect(remote.GetLabels()).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
					Expect(remote.GetLabels()).To(HaveKeyWithValue("app", "demo"))
					Expect(remote.GetLabels()).ToNot(HaveKey(FakeNotReflectedLabelKey))
					Expect(remote.GetAnnotations()).To(HaveKeyWithValue("anno", "val"))
					Expect(remote.GetAnnotations()).ToNot(HaveKey(FakeNotReflectedAnnotKey))

					spec, found, specErr := unstructured.NestedMap(remote.Object, "spec")
					Expect(specErr).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(spec).To(HaveKeyWithValue("replicas", int64(3)))
				})
			})

			When("remote create is forbidden", func() {
				BeforeEach(func() {
					remoteClient.(*dynamicfake.FakeDynamicClient).PrependReactor("create", "widgets",
						func(_ clienttesting.Action) (bool, runtime.Object, error) {
							return true, nil, kerrors.NewForbidden(gvr.GroupResource(), WidgetName, fmt.Errorf("denied"))
						})
				})

				It("should soft-fail without creating a remote twin", func() {
					Expect(err).ToNot(HaveOccurred())
					_, getErr := remoteClient.Resource(gvr).Namespace(RemoteNamespace).Get(ctx, WidgetName, metav1.GetOptions{})
					Expect(getErr).To(BeNotFound())
				})
			})

			When("remote create returns not found (CRD missing)", func() {
				BeforeEach(func() {
					remoteClient.(*dynamicfake.FakeDynamicClient).PrependReactor("create", "widgets",
						func(_ clienttesting.Action) (bool, runtime.Object, error) {
							return true, nil, kerrors.NewNotFound(gvr.GroupResource(), WidgetName)
						})
				})

				It("should soft-fail without creating a remote twin", func() {
					Expect(err).ToNot(HaveOccurred())
					_, getErr := remoteClient.Resource(gvr).Namespace(RemoteNamespace).Get(ctx, WidgetName, metav1.GetOptions{})
					Expect(getErr).To(BeNotFound())
				})
			})

			When("the remote object exists and is reflected", func() {
				BeforeEach(func() {
					CreateRemote(newWidget(WidgetName, RemoteNamespace, forge.ReflectionLabels(), nil,
						map[string]interface{}{"replicas": int64(1)},
						map[string]interface{}{"ready": true}))
				})

				It("should succeed and update the remote spec", func() {
					Expect(err).ToNot(HaveOccurred())
					remote := GetRemote(WidgetName)
					spec, _, _ := unstructured.NestedMap(remote.Object, "spec")
					Expect(spec).To(HaveKeyWithValue("replicas", int64(3)))
				})

				It("should sync status from remote to local", func() {
					Expect(err).ToNot(HaveOccurred())
					local := GetLocal(WidgetName)
					status, found, statusErr := unstructured.NestedMap(local.Object, "status")
					Expect(statusErr).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(status).To(HaveKeyWithValue("ready", true))
				})
			})

			When("local status update is method not supported", func() {
				BeforeEach(func() {
					CreateRemote(newWidget(WidgetName, RemoteNamespace, forge.ReflectionLabels(), nil,
						map[string]interface{}{"replicas": int64(1)},
						map[string]interface{}{"ready": true}))
					localClient.(*dynamicfake.FakeDynamicClient).PrependReactor("update", "widgets/status",
						func(_ clienttesting.Action) (bool, runtime.Object, error) {
							return true, nil, kerrors.NewMethodNotSupported(gvr.GroupResource(), "update")
						})
				})

				It("should succeed, update remote spec, and skip status sync", func() {
					Expect(err).ToNot(HaveOccurred())
					remote := GetRemote(WidgetName)
					spec, _, _ := unstructured.NestedMap(remote.Object, "spec")
					Expect(spec).To(HaveKeyWithValue("replicas", int64(3)))

					local := GetLocal(WidgetName)
					_, found, statusErr := unstructured.NestedMap(local.Object, "status")
					Expect(statusErr).ToNot(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})

			When("the remote object exists but is not managed by us", func() {
				BeforeEach(func() {
					CreateRemote(newWidget(WidgetName, RemoteNamespace,
						map[string]string{"foreign": "true"}, nil,
						map[string]interface{}{"replicas": int64(9)}, nil))
				})

				It("should succeed without modifying the remote object", func() {
					Expect(err).ToNot(HaveOccurred())
					remote := GetRemote(WidgetName)
					spec, _, _ := unstructured.NestedMap(remote.Object, "spec")
					Expect(spec).To(HaveKeyWithValue("replicas", int64(9)))
					Expect(remote.GetLabels()).To(HaveKeyWithValue("foreign", "true"))
				})
			})
		})

		When("the local object exists without allow annotation (AllowList)", func() {
			BeforeEach(func() {
				CreateLocal(newWidget(WidgetName, LocalNamespace, nil, nil,
					map[string]interface{}{"replicas": int64(1)}, nil))
			})

			It("should succeed without creating a remote twin", func() {
				Expect(err).ToNot(HaveOccurred())
				_, getErr := remoteClient.Resource(gvr).Namespace(RemoteNamespace).Get(ctx, WidgetName, metav1.GetOptions{})
				Expect(getErr).To(BeNotFound())
			})

			When("a remote twin already exists", func() {
				BeforeEach(func() {
					CreateRemote(newWidget(WidgetName, RemoteNamespace, forge.ReflectionLabels(), nil,
						map[string]interface{}{"replicas": int64(1)}, nil))
				})

				It("should delete the remote twin", func() {
					Expect(err).ToNot(HaveOccurred())
					_, getErr := remoteClient.Resource(gvr).Namespace(RemoteNamespace).Get(ctx, WidgetName, metav1.GetOptions{})
					Expect(getErr).To(BeNotFound())
				})
			})
		})
	})
})
