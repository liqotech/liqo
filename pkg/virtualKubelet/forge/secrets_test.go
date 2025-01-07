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

package forge_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

var _ = Describe("Secrets Forging", func() {
	Describe("the RemoteSecret function", func() {
		var (
			input       *corev1.Secret
			output      *corev1apply.SecretApplyConfiguration
			forgingOpts *forge.ForgingOpts
		)

		BeforeEach(func() {
			input = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "name", Namespace: "original",
					Labels:      map[string]string{"foo": "bar", testutil.FakeNotReflectedLabelKey: "true"},
					Annotations: map[string]string{"bar": "baz", testutil.FakeNotReflectedAnnotKey: "true"},
				},
				Data:      map[string][]byte{"data-key": []byte("ABC")},
				Type:      corev1.SecretTypeBasicAuth,
				Immutable: pointer.Bool(true),
			}

			forgingOpts = testutil.FakeForgingOpts()
		})

		JustBeforeEach(func() { output = forge.RemoteSecret(input, "reflected", forgingOpts) })

		It("should correctly set the name and namespace", func() {
			Expect(output.Name).To(PointTo(Equal("name")))
			Expect(output.Namespace).To(PointTo(Equal("reflected")))
		})

		It("should correctly set the labels", func() {
			Expect(output.Labels).To(HaveKeyWithValue("foo", "bar"))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(RemoteClusterID)))
			Expect(output.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
		})

		It("should correctly set the annotations", func() {
			Expect(output.Annotations).To(HaveKeyWithValue("bar", "baz"))
			Expect(output.Annotations).ToNot(HaveKey(testutil.FakeNotReflectedAnnotKey))
		})

		It("should correctly set the data", func() {
			Expect(output.Data).NotTo(BeNil())
			Expect(output.Data).To(HaveKeyWithValue("data-key", []byte("ABC")))
		})

		It("should correctly set the type", func() {
			Expect(output.Type).To(PointTo(Equal(corev1.SecretTypeBasicAuth)))
		})

		It("should correctly set the immutable field", func() {
			Expect(output.Immutable).NotTo(BeNil())
			Expect(output.Immutable).To(PointTo(BeTrue()))
		})

		When("it is of type ServiceAccountToken", func() {
			BeforeEach(func() {
				input.Type = corev1.SecretTypeServiceAccountToken
				input.Annotations[corev1.ServiceAccountNameKey] = "service-account"
			})

			It("should change the type to Opaque", func() {
				Expect(output.Type).To(PointTo(Equal(corev1.SecretTypeOpaque)))
			})

			It("should add a label with the ServiceAccount name", func() {
				Expect(output.Labels).To(HaveLen(4)) // Ensure existing labels are not removed
				Expect(output.Labels).To(HaveKeyWithValue(corev1.ServiceAccountNameKey, "service-account"))
			})
		})
	})
})

var _ = Describe("Service accounts management", func() {
	var (
		now    time.Time
		tokens forge.ServiceAccountPodTokens
	)

	Describe("the IsServiceAccountSecret function", func() {
		var (
			labels map[string]string
			result bool
		)

		JustBeforeEach(func() {
			result = forge.IsServiceAccountSecret(
				&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: labels}})
		})

		When("all labels match", func() {
			BeforeEach(func() {
				labels = map[string]string{forge.LiqoSASecretForPodNameKey: "foo", forge.LiqoSASecretForServiceAccountKey: "bar"}
			})

			It("should return a positive value", func() { Expect(result).To(BeTrue()) })
		})

		When("only one label matches", func() {
			BeforeEach(func() {
				labels = map[string]string{forge.LiqoSASecretForPodNameKey: "foo", "other": "bar"}
			})

			It("should return a negative value", func() { Expect(result).To(BeFalse()) })
		})

		When("no label matches", func() {
			BeforeEach(func() {
				labels = map[string]string{"other": "foo", "existing": "bar"}
			})

			It("should return a positive value", func() { Expect(result).To(BeFalse()) })
		})
	})

	BeforeEach(func() {
		now = time.Now()
		tokens = forge.ServiceAccountPodTokens{
			PodName:            "pod",
			PodUID:             types.UID("pod-uid"),
			ServiceAccountName: "sa",

			Tokens: []*forge.ServiceAccountPodToken{
				{Key: "key1", Token: "tkn1", ExpirationSeconds: 3600, ActualExpiration: now.Add(3600 * time.Second), Audiences: []string{"aud1"}},
				{Key: "key2", Token: "tkn2", ExpirationSeconds: 5000, ActualExpiration: now.Add(1600 * time.Second), Audiences: []string{"aud2"}},
				{Key: "key3", Token: "tkn3", ExpirationSeconds: 1800, ActualExpiration: now.Add(1800 * time.Second), Audiences: []string{"aud3"}},
			},
		}
	})

	Describe("the RemoteServiceAccountSecret function", func() {
		var output *corev1apply.SecretApplyConfiguration

		JustBeforeEach(func() { output = forge.RemoteServiceAccountSecret(&tokens, "name", "namespace", "fakenode") })

		It("should correctly set the name and namespace", func() {
			Expect(output.Name).To(PointTo(Equal("name")))
			Expect(output.Namespace).To(PointTo(Equal("namespace")))
		})

		It("should correctly set the labels", func() {
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, string(LocalClusterID)))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, string(RemoteClusterID)))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoSASecretForPodNameKey, "pod"))
			Expect(output.Labels).To(HaveKeyWithValue(forge.LiqoSASecretForServiceAccountKey, "sa"))
		})

		It("should correctly set the annotations", func() {
			Expect(output.Annotations).To(HaveKeyWithValue(forge.LiqoSASecretForPodUIDKey, "pod-uid"))
			Expect(output.Annotations).To(HaveKeyWithValue(
				forge.LiqoSASecretExpirationKey, time.Now().Add(1600*time.Second).Format(time.RFC3339)))
		})

		It("should correctly set the data", func() {
			Expect(output.StringData).NotTo(BeNil())
			Expect(output.StringData).To(HaveKeyWithValue("key1", "tkn1"))
			Expect(output.StringData).To(HaveKeyWithValue("key2", "tkn2"))
			Expect(output.StringData).To(HaveKeyWithValue("key3", "tkn3"))
		})

		It("should correctly set the type", func() {
			Expect(output.Type).To(PointTo(Equal(corev1.SecretTypeOpaque)))
		})

		It("should correctly set the immutable field", func() {
			Expect(output.Immutable).To(PointTo(BeFalse()))
		})
	})

	Describe("the ServiceAccountToken*FromSecret functions", func() {
		var input *corev1.Secret

		BeforeEach(func() {
			input = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "name", Namespace: "namespace"},
				Data:       map[string][]byte{"key1": []byte("tkn1"), "key2": []byte("tkn2")},
			}
		})

		Context("retrieving the token associated with a key", func() {
			var (
				key    string
				output string
			)

			JustBeforeEach(func() { output = forge.ServiceAccountTokenFromSecret(input, key) })

			When("the secret is nil", func() {
				BeforeEach(func() { input = nil })
				It("should return an empty string", func() { Expect(output).To(BeEmpty()) })
			})

			When("the key does not exist", func() {
				BeforeEach(func() { key = "non-existing" })
				It("should return an empty string", func() { Expect(output).To(BeEmpty()) })
			})

			When("the key exists", func() {
				BeforeEach(func() { key = "key2" })
				It("should return the corresponding token", func() { Expect(output).To(Equal("tkn2")) })
			})
		})

		Context("retrieving the pod UID associated with the tokens", func() {
			var (
				podUID types.UID
				output types.UID
			)

			BeforeEach(func() { podUID = "pod-uid" })
			JustBeforeEach(func() { output = forge.ServiceAccountPodUIDFromSecret(input, podUID) })

			When("the secret is nil", func() {
				BeforeEach(func() { input = nil })
				It("should return the pod UID", func() { Expect(output).To(BeIdenticalTo(types.UID("pod-uid"))) })
			})

			When("the pod UID annotation does not exist", func() {
				It("should return an empty pod UID", func() { Expect(output).To(BeEmpty()) })
			})

			When("the pod UID annotation exists", func() {
				BeforeEach(func() { input.Annotations = map[string]string{forge.LiqoSASecretForPodUIDKey: "existing-pod-uid"} })
				It("should return the corresponding UID", func() { Expect(output).To(BeIdenticalTo(types.UID("existing-pod-uid"))) })
			})
		})

		Context("retrieving the expiration associated with the tokens", func() {
			var output time.Time

			JustBeforeEach(func() { output = forge.ServiceAccountTokenExpirationFromSecret(input) })

			When("the secret is nil", func() {
				BeforeEach(func() { input = nil })
				It("should return a zero timestamp", func() { Expect(output).To(BeZero()) })
			})

			When("the expiration annotation does not exist", func() {
				It("should return a zero timestamp", func() { Expect(output).To(BeZero()) })
			})

			When("the expiration annotation exists, but has an invalid value", func() {
				BeforeEach(func() { input.Annotations = map[string]string{forge.LiqoSASecretExpirationKey: "invalid"} })
				It("should return a zero timestamp", func() { Expect(output).To(BeZero()) })
			})

			When("the expiration annotation exists, and has a valid value", func() {
				BeforeEach(func() {
					input.Annotations = map[string]string{forge.LiqoSASecretExpirationKey: "2022-11-09T15:08:11Z"}
				})

				It("should return the corresponding expiration timestamp", func() {
					Expect(output).To(Equal(time.Date(2022, time.November, 9, 15, 8, 11, 0, time.UTC)))
				})
			})
		})
	})

	Describe("the AddToken function", func() {
		JustBeforeEach(func() { tokens.AddToken("new-key", "audience", 1000) })

		It("should correctly generate and add the entry", func() {
			Expect(tokens.Tokens).To(ContainElement(&forge.ServiceAccountPodToken{
				Key: "new-key", Audiences: []string{"audience"}, ExpirationSeconds: 1000,
			}))
		})
	})

	Describe("the EarliestExpiration function", func() {
		var output time.Time
		JustBeforeEach(func() { output = tokens.EarliestExpiration() })
		It("should correctly return the expiration timestamp", func() { Expect(output).To(Equal(now.Add(1600 * time.Second))) })
	})

	Describe("the EarliestRefresh function", func() {
		var output time.Time
		JustBeforeEach(func() { output = tokens.EarliestRefresh() })
		It("should correctly return the refresh timestamp", func() { Expect(output).To(Equal(now.Add(600 * time.Second))) })
	})

	Describe("the TokenRequest function", func() {
		var (
			pod    corev1.Pod
			output *authenticationv1.TokenRequest
		)

		BeforeEach(func() { pod = corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "name", UID: types.UID("uid")}} })
		JustBeforeEach(func() { output = tokens.Tokens[0].TokenRequest(&pod) })

		It("should return a non-nil output", func() { Expect(output).ToNot(BeNil()) })
		It("should set the correct audience", func() { Expect(output.Spec.Audiences).To(Equal([]string{"aud1"})) })
		It("should set the correct expiration", func() { Expect(output.Spec.ExpirationSeconds).To(PointTo(BeNumerically("==", 3600))) })
		It("should set the correct reference", func() {
			Expect(output.Spec.BoundObjectRef).To(Equal(
				&authenticationv1.BoundObjectReference{APIVersion: "v1", Kind: "Pod", Name: "name", UID: types.UID("uid")}))
		})
	})
})
