// Copyright 2019-2022 The Liqo Authors
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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/resources"
	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("Manager tests", func() {

	const (
		localNamespace  = "foo"
		remoteNamespace = "bar"
		localClusterID  = "local-id"
		remoteClusterID = "remote-id"
		workers         = 2
	)

	var (
		manager *Manager

		local, remote dynamic.Interface
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(netv1alpha1.AddToScheme(scheme))

		local = fake.NewSimpleDynamicClient(scheme)
		remote = fake.NewSimpleDynamicClient(scheme)
	})

	JustBeforeEach(func() { manager = NewManager(local, localClusterID, workers, 1*time.Hour) })

	Describe("the NewManager function", func() {
		It("Should return a non nil manager", func() { Expect(manager).ToNot(BeNil()) })
		It("Should correctly populate the manager fields", func() {
			Expect(manager.client).To(Equal(local))

			Expect(manager.resync).To(Equal(1 * time.Hour))
			Expect(manager.clusterID).To(BeIdenticalTo(localClusterID))
			Expect(manager.workers).To(BeNumerically("==", workers))

			Expect(manager.listers).ToNot(BeNil())
			Expect(manager.handlers).ToNot(BeNil())
		})
	})

	Describe("the NewForRemote function", func() {
		var reflector *Reflector

		JustBeforeEach(func() { reflector = manager.NewForRemote(remote, remoteClusterID, localNamespace, remoteNamespace) })
		It("Should return a non nil reflector", func() { Expect(reflector).ToNot(BeNil()) })
		It("Should correctly reference the parent manager", func() { Expect(reflector.manager).To(Equal(manager)) })
		It("Should correctly populate the reflector fields", func() {
			Expect(reflector.localNamespace).To(BeIdenticalTo(localNamespace))
			Expect(reflector.localClusterID).To(BeIdenticalTo(localClusterID))

			Expect(reflector.remoteClient).To(Equal(remote))
			Expect(reflector.remoteNamespace).To(BeIdenticalTo(remoteNamespace))
			Expect(reflector.remoteClusterID).To(BeIdenticalTo(remoteClusterID))

			Expect(reflector.resources).ToNot(BeNil())
			Expect(reflector.workqueue).ToNot(BeNil())
		})
	})

	Describe("the Start function", func() {
		const objName = "object"

		var (
			ctx    context.Context
			cancel context.CancelFunc

			gvr schema.GroupVersionResource
			res []resources.Resource

			objNamespace string
			objGVR       schema.GroupVersionResource
			objGVK       schema.GroupVersionKind
			receiver     chan item
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())
			gvr = netv1alpha1.NetworkConfigGroupVersionResource
			res = []resources.Resource{{GroupVersionResource: gvr}}

			objNamespace = localNamespace
			objGVK = netv1alpha1.GroupVersion.WithKind("NetworkConfig")
			objGVR = netv1alpha1.NetworkConfigGroupVersionResource

			receiver = make(chan item, 1)
		})

		AfterEach(func() { cancel() })

		CreateNetworkConfig := func() error {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(objGVK)
			obj.SetNamespace(objNamespace)
			obj.SetName(objName)
			obj.SetLabels(map[string]string{
				consts.ReplicationRequestedLabel:   strconv.FormatBool(true),
				consts.ReplicationDestinationLabel: remoteClusterID,
			})
			_, err := local.Resource(objGVR).Namespace(objNamespace).Create(ctx, obj, metav1.CreateOptions{})
			return err
		}

		ContextBody := func(alreadyPresent bool) func() {
			return func() {
				JustBeforeEach(func() {
					if alreadyPresent {
						Expect(CreateNetworkConfig()).To(Succeed())
					}

					manager.Start(ctx, res)
					manager.registerHandler(gvr, localNamespace, func(key item) { receiver <- key })

					if !alreadyPresent {
						Expect(CreateNetworkConfig()).To(Succeed())
					}
				})

				When("the object matches the namespace and GVR of the registered handler", func() {
					It("should trigger the handler with the correct item", func() {
						Eventually(receiver).Should(Receive(Equal(item{gvr: objGVR, name: objName})))
					})

					if !alreadyPresent {
						When("the handler is then unregistered", func() {
							JustBeforeEach(func() { manager.unregisterHandler(gvr, localNamespace) })
							It("should not trigger the handler", func() { Consistently(receiver).ShouldNot(Receive()) })
						})
					}
				})

				When("the object matches the namespace but not the GVR of the registered handler", func() {
					BeforeEach(func() {
						objGVK = netv1alpha1.GroupVersion.WithKind("TunnelEndpoint")
						objGVR = netv1alpha1.TunnelEndpointGroupVersionResource
					})
					It("should not trigger the handler", func() { Consistently(receiver).ShouldNot(Receive()) })
				})

				When("the object matches the GVR but not the namespace of the registered handler", func() {
					BeforeEach(func() { objNamespace = "something-else" })
					It("should not trigger the handler", func() { Consistently(receiver).ShouldNot(Receive()) })
				})
			}
		}

		Context("the object is created before having started the manager and registered the handler", ContextBody(true))
		Context("the object is created after having started the manager and registered the handler", ContextBody(false))
	})
})
