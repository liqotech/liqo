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

package netcfgcreator

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
)

var _ = Describe("Service Watcher functions", func() {
	var (
		handled chan struct{}

		sw      *ServiceWatcher
		service corev1.Service
	)

	BeforeEach(func() {
		handled = make(chan struct{})
		sw = NewServiceWatcher(func(rli workqueue.RateLimitingInterface) { close(handled) })
		service = corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"}}
	})

	Describe("The handle function", func() {
		JustBeforeEach(func() { sw.handle(&service, nil) })

		Context("node port service", func() {
			BeforeEach(func() {
				service.Annotations = map[string]string{"net.liqo.io/gatewayNodeIP": "1.1.1.1"}
				service.Spec.Type = corev1.ServiceTypeNodePort
				service.Spec.Ports = []corev1.ServicePort{{Name: "wireguard", NodePort: 9999}}
			})

			When("given a valid service", func() {
				It("should retrieve the correct endpoint", func() {
					ip, port := sw.WiregardEndpoint()
					Expect(ip).To(BeIdenticalTo("1.1.1.1"))
					Expect(port).To(BeIdenticalTo("9999"))
				})
				It("should execute the handle function", func() { Expect(handled).To(BeClosed()) })
				It("should be initialized", func() { Expect(sw.configured).To(BeTrue()) })
			})

			When("given an invalid service (missing the annotation)", func() {
				BeforeEach(func() { service.Annotations = nil })
				It("should not execute the handle function", func() { Expect(handled).ToNot(BeClosed()) })
				It("should not be initialized", func() { Expect(sw.configured).To(BeFalse()) })
			})

			When("given an invalid service (missing the port)", func() {
				BeforeEach(func() { service.Spec.Ports = nil })
				It("should not execute the handle function", func() { Expect(handled).ToNot(BeClosed()) })
				It("should not be initialized", func() { Expect(sw.configured).To(BeFalse()) })
			})

			When("given an invalid service (missing the node port)", func() {
				BeforeEach(func() { service.Spec.Ports[0].NodePort = 0 })
				It("should not execute the handle function", func() { Expect(handled).ToNot(BeClosed()) })
				It("should not be initialized", func() { Expect(sw.configured).To(BeFalse()) })
			})
		})

		Context("load balancer service", func() {
			BeforeEach(func() {
				service.Spec.Type = corev1.ServiceTypeLoadBalancer
				service.Spec.Ports = []corev1.ServicePort{{Name: "wireguard", Port: 9999}}
				service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "1.1.1.1"}}
			})

			When("given a valid service (with IP)", func() {
				It("should retrieve the correct endpoint", func() {
					ip, port := sw.WiregardEndpoint()
					Expect(ip).To(BeIdenticalTo("1.1.1.1"))
					Expect(port).To(BeIdenticalTo("9999"))
				})
				It("should execute the handle function", func() { Expect(handled).To(BeClosed()) })
				It("should be initialized", func() { Expect(sw.configured).To(BeTrue()) })
			})

			When("given a valid service (with hostname)", func() {
				BeforeEach(func() {
					service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{Hostname: "foo.example.com"}}
				})
				It("should retrieve the correct endpoint", func() {
					ip, port := sw.WiregardEndpoint()
					Expect(ip).To(BeIdenticalTo("foo.example.com"))
					Expect(port).To(BeIdenticalTo("9999"))
				})
				It("should execute the handle function", func() { Expect(handled).To(BeClosed()) })
				It("should be initialized", func() { Expect(sw.configured).To(BeTrue()) })
			})

			When("given an invalid service (missing the load balancer ingress)", func() {
				BeforeEach(func() { service.Status.LoadBalancer.Ingress = nil })
				It("should not execute the handle function", func() { Expect(handled).ToNot(BeClosed()) })
				It("should not be initialized", func() { Expect(sw.configured).To(BeFalse()) })
			})

			When("given an invalid service (missing the port)", func() {
				BeforeEach(func() { service.Spec.Ports = nil })
				It("should not execute the handle function", func() { Expect(handled).ToNot(BeClosed()) })
				It("should not be initialized", func() { Expect(sw.configured).To(BeFalse()) })
			})
		})

		Context("cluster IP service", func() {
			BeforeEach(func() { service.Spec.Type = corev1.ServiceTypeClusterIP })
			It("should not execute the handle function", func() { Expect(handled).ToNot(BeClosed()) })
			It("should not be initialized", func() { Expect(sw.configured).To(BeFalse()) })
		})

		Context("external name service", func() {
			BeforeEach(func() { service.Spec.Type = corev1.ServiceTypeExternalName })
			It("should not execute the handle function", func() { Expect(handled).ToNot(BeClosed()) })
			It("should not be initialized", func() { Expect(sw.configured).To(BeFalse()) })
		})
	})

	Describe("The WaitForConfigured function", func() {
		var (
			ctx    context.Context
			cancel context.CancelFunc
		)

		BeforeEach(func() { ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond) })
		AfterEach(func() { cancel() })

		When("already initialized", func() {
			BeforeEach(func() {
				sw.configured = true
				close(sw.wait)
			})

			It("should return immediately true", func() {
				start := time.Now()
				Expect(sw.WaitForConfigured(ctx)).To(BeTrue())
				Expect(time.Now()).To(BeTemporally("~", start, time.Millisecond))
			})
		})

		When("not initialized", func() {
			It("should return false when the timeout expires", func() {
				start := time.Now()
				Expect(sw.WaitForConfigured(ctx)).To(BeFalse())
				Expect(time.Now()).To(BeTemporally(">", start, 50*time.Millisecond))
			})
		})

		When("initialized while waiting", func() {
			BeforeEach(func() {
				go func() {
					time.Sleep(10 * time.Millisecond)
					sw.configured = true
					close(sw.wait)
				}()
			})

			It("should return true when initialized", func() {
				start := time.Now()
				Expect(sw.WaitForConfigured(ctx)).To(BeTrue())
				Expect(time.Now()).To(BeTemporally(">", start, 10*time.Millisecond))
			})
		})
	})
})
