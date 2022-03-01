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

	"github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("Secret Watcher functions", func() {
	var (
		handled chan struct{}

		sw     *SecretWatcher
		secret corev1.Secret
	)

	BeforeEach(func() {
		handled = make(chan struct{})
		sw = NewSecretWatcher(func(rli workqueue.RateLimitingInterface) { close(handled) })
		secret = corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "bar"}}
	})

	Describe("The handle function", func() {
		const key = "cHVibGljLWtleS1vZi10aGUtY29ycmVjdC1sZW5ndGg="

		JustBeforeEach(func() { sw.handle(&secret, nil) })

		When("given a valid secret", func() {
			BeforeEach(func() {
				secret.Data = map[string][]byte{consts.PublicKey: []byte(key)}
			})

			When("not yet initialized", func() {
				It("should retrieve the correct public key", func() { Expect(sw.WiregardPublicKey()).To(BeIdenticalTo(key)) })
				It("should execute the handle function", func() { Expect(handled).To(BeClosed()) })
				It("should be initialized", func() { Expect(sw.configured).To(BeTrue()) })
			})

			When("already initialized", func() {
				BeforeEach(func() {
					sw.wiregardPublicKey = "previous-public-key"
					sw.configured = true
				})

				It("should retrieve the correct public key", func() { Expect(sw.WiregardPublicKey()).To(BeIdenticalTo(key)) })
				It("should execute the handle function", func() { Expect(handled).To(BeClosed()) })
				It("should be initialized", func() { Expect(sw.configured).To(BeTrue()) })
			})
		})

		When("given an invalid secret", func() {
			BeforeEach(func() {
				secret.Data = map[string][]byte{"incorrect-key": []byte(key)}
			})

			It("should leave the public key unmodified", func() { Expect(sw.WiregardPublicKey()).To(BeIdenticalTo("")) })
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
