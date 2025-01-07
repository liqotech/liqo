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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	. "github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ = Describe("NamespacedReflector tests", func() {
	const (
		localNamespace  = "local"
		remoteNamespace = "remote"
		name            = "foo"
	)

	Context("the NewNamespacedReflector function", func() {
		var (
			nsrfl          NamespacedReflector
			ready          bool
			forgingOpts    *forge.ForgingOpts
			reflectionType offloadingv1beta1.ReflectionType
		)

		BeforeEach(func() {
			ready = false
			forgingOpts = &forge.ForgingOpts{}
			reflectionType = offloadingv1beta1.CustomLiqo
		})

		JustBeforeEach(func() {
			opts := options.NamespacedOpts{
				LocalNamespace: localNamespace, RemoteNamespace: remoteNamespace,
				Ready: func() bool { return ready }, EventBroadcaster: record.NewBroadcaster(),
				ReflectionType: reflectionType, ForgingOpts: forgingOpts,
			}
			nsrfl = NewNamespacedReflector(&opts, name)
		})

		It("should correctly initialize the namespaced reflector", func() {
			Expect(nsrfl.EventRecorder).ToNot(BeNil())
			Expect(nsrfl.local).To(BeIdenticalTo(localNamespace))
			Expect(nsrfl.remote).To(BeIdenticalTo(remoteNamespace))
			Expect(nsrfl.ready).ToNot(BeNil())
			Expect(nsrfl.reflectionType).To(BeIdenticalTo(reflectionType))
			Expect(nsrfl.ForgingOpts).To(BeIdenticalTo(forgingOpts))
		})

		Context("the readiness property", func() {
			When("the namespaced reflector is not ready", func() {
				It("Ready should return false", func() { Expect(nsrfl.Ready()).To(BeFalse()) })
			})
			When("the namespaced reflector is ready", func() {
				JustBeforeEach(func() { ready = true })
				It("Ready should return true", func() { Expect(nsrfl.Ready()).To(BeTrue()) })
			})
		})

		Context("the utility functions", func() {
			It("LocalNamespace should return the local namespace", func() {
				Expect(nsrfl.LocalNamespace()).To(BeIdenticalTo(localNamespace))
			})
			It("RemoteNamespace should return the remote namespace", func() {
				Expect(nsrfl.RemoteNamespace()).To(BeIdenticalTo(remoteNamespace))
			})

			It("LocalRef should return a local namespaced name", func() {
				Expect(nsrfl.LocalRef(name)).To(BeIdenticalTo(klog.KRef(localNamespace, name)))
			})
			It("RemoteRef should return a local namespaced name", func() {
				Expect(nsrfl.RemoteRef(name)).To(BeIdenticalTo(klog.KRef(remoteNamespace, name)))
			})
		})

		Context("remote resource deletion", func() {
			var (
				ctx     context.Context
				client  corev1clients.ServiceInterface
				service corev1.Service

				err error
			)

			JustBeforeEach(func() {
				ctx = context.Background()
				client = fake.NewSimpleClientset(&service).CoreV1().Services(localNamespace)
				err = nsrfl.DeleteRemote(ctx, client, "Service", name, types.UID("discarded-by-fake-client"))
			})

			When("the object does not already exist", func() {
				It("should not return an error", func() { Expect(err).ToNot(HaveOccurred()) })
			})
			When("the object does exist", func() {
				BeforeEach(func() { service = corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: localNamespace}} })
				It("should not return an error", func() { Expect(err).ToNot(HaveOccurred()) })
				It("should have correctly deleted the object", func() {
					_, err = client.Get(ctx, name, metav1.GetOptions{})
					Expect(err).To(BeNotFound())
				})
			})
		})
	})
})
