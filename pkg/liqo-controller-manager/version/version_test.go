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

package version

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestVersion(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Version Suite")
}

var _ = Describe("GetVersionFromDeployment", func() {
	var (
		ctx       context.Context
		clientset *fake.Clientset
		namespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientset = fake.NewSimpleClientset()
		namespace = "liqo"
	})

	It("should extract version from deployment image tag", func() {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "liqo-controller-manager",
				Namespace: namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "controller-manager",
								Image: "ghcr.io/liqotech/liqo-controller-manager:v0.10.3",
							},
						},
					},
				},
			},
		}
		_, err := clientset.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		version := GetVersionFromDeployment(ctx, clientset, namespace, "liqo-controller-manager")
		Expect(version).To(Equal("v0.10.3"))
	})

	It("should return empty string when deployment doesn't exist", func() {
		version := GetVersionFromDeployment(ctx, clientset, namespace, "nonexistent")
		Expect(version).To(BeEmpty())
	})

	It("should return empty string when no containers in deployment", func() {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "liqo-controller-manager",
				Namespace: namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{},
					},
				},
			},
		}
		_, err := clientset.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		version := GetVersionFromDeployment(ctx, clientset, namespace, "liqo-controller-manager")
		Expect(version).To(BeEmpty())
	})

	It("should return empty string when image has no tag", func() {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "liqo-controller-manager",
				Namespace: namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "controller-manager",
								Image: "ghcr.io/liqotech/liqo-controller-manager",
							},
						},
					},
				},
			},
		}
		_, err := clientset.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		version := GetVersionFromDeployment(ctx, clientset, namespace, "liqo-controller-manager")
		Expect(version).To(BeEmpty())
	})
})

var _ = Describe("QueryRemoteVersion", func() {
	It("should return error when API server URL is empty", func() {
		ctx := context.Background()
		_, err := QueryRemoteVersion(ctx, "", "token", "liqo")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("API server URL is required"))
	})

	It("should return error when token is empty", func() {
		ctx := context.Background()
		_, err := QueryRemoteVersion(ctx, "https://example.com", "", "liqo")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("authentication token is required"))
	})

	It("should return error when namespace is empty", func() {
		ctx := context.Background()
		_, err := QueryRemoteVersion(ctx, "https://example.com", "token", "")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Liqo namespace is required"))
	})
})

var _ = Describe("GetLocalVersion", func() {
	var (
		ctx       context.Context
		clientset *fake.Clientset
		namespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientset = fake.NewSimpleClientset()
		namespace = "liqo"
	})

	It("should retrieve version from ConfigMap", func() {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      LiqoVersionConfigMapName,
				Namespace: namespace,
			},
			Data: map[string]string{
				LiqoVersionKey: "v0.10.3",
			},
		}
		_, err := clientset.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		version, err := GetLocalVersion(ctx, clientset, namespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(version).To(Equal("v0.10.3"))
	})

	It("should return error when ConfigMap doesn't exist", func() {
		_, err := GetLocalVersion(ctx, clientset, namespace)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to get local version ConfigMap"))
	})

	It("should return error when version key is missing", func() {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      LiqoVersionConfigMapName,
				Namespace: namespace,
			},
			Data: map[string]string{},
		}
		_, err := clientset.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		_, err = GetLocalVersion(ctx, clientset, namespace)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("version key not found"))
	})
})

var _ = Describe("GetRemoteVersion", func() {
	var (
		ctx       context.Context
		clientset *fake.Clientset
		namespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientset = fake.NewSimpleClientset()
		namespace = "liqo"
	})

	It("should retrieve version from remote ConfigMap", func() {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      LiqoVersionConfigMapName,
				Namespace: namespace,
			},
			Data: map[string]string{
				LiqoVersionKey: "v0.10.3",
			},
		}
		_, err := clientset.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		version := GetRemoteVersion(ctx, clientset, namespace)
		Expect(version).To(Equal("v0.10.3"))
	})

	It("should return empty string when ConfigMap doesn't exist", func() {
		version := GetRemoteVersion(ctx, clientset, namespace)
		Expect(version).To(BeEmpty())
	})

	It("should return empty string when version key is missing", func() {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      LiqoVersionConfigMapName,
				Namespace: namespace,
			},
			Data: map[string]string{},
		}
		_, err := clientset.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		version := GetRemoteVersion(ctx, clientset, namespace)
		Expect(version).To(BeEmpty())
	})
})

var _ = Describe("GetVersionReaderToken", func() {
	var (
		ctx       context.Context
		clientset *fake.Clientset
		namespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		clientset = fake.NewSimpleClientset()
		namespace = "liqo"
	})

	It("should retrieve token from secret", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      LiqoVersionReaderSecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeServiceAccountToken,
			Data: map[string][]byte{
				corev1.ServiceAccountTokenKey: []byte("test-token-value"),
			},
		}
		_, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		token, err := GetVersionReaderToken(ctx, clientset, namespace)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).To(Equal("test-token-value"))
	})

	It("should return error when secret doesn't exist", func() {
		_, err := GetVersionReaderToken(ctx, clientset, namespace)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unable to get version reader secret"))
	})

	It("should return error when token key is missing", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      LiqoVersionReaderSecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeServiceAccountToken,
			Data: map[string][]byte{},
		}
		_, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		_, err = GetVersionReaderToken(ctx, clientset, namespace)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("token not found or empty"))
	})

	It("should return error when token is empty", func() {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      LiqoVersionReaderSecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeServiceAccountToken,
			Data: map[string][]byte{
				corev1.ServiceAccountTokenKey: []byte(""),
			},
		}
		_, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())

		_, err = GetVersionReaderToken(ctx, clientset, namespace)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("token not found or empty"))
	})
})
