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

package generate

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakecontroller "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/consts"
)

const (
	liqoNamespace  = "liqo"
	token          = "my-token"
	commandName    = "liqoctl"
	authEndpoint   = "1.1.1.1"
	clusterName    = "test-cluster"
	localClusterID = "local-cluster-id"
)

var (
	k8sClient client.Client
	ctx       context.Context
)

func TestAddCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Parameters Fetching")
}

var _ = Describe("Test the generate command works as expected", func() {

	setUpEnvironment := func(liqonamespace, localClusterID, localClusterName, token string,
		deployArgs []string) client.Client {
		// Create Namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: liqonamespace,
			},
		}
		// Create ClusterID ConfigMap
		clusterIDConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "clusterid-configmap",
				Namespace: liqonamespace,
				Labels: map[string]string{
					"app.kubernetes.io/component": "clusterid-configmap",
					"app.kubernetes.io/name":      "clusterid-configmap",
				},
			},
			Data: map[string]string{
				consts.ClusterIDConfigMapKey:   localClusterID,
				consts.ClusterNameConfigMapKey: localClusterName,
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      auth.TokenSecretName,
				Namespace: liqoNamespace,
			},
			Data: map[string][]byte{
				"token": []byte(token),
			},
		}

		discoveryDeploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "whatever",
				Namespace: liqoNamespace,
				Labels: map[string]string{
					"app.kubernetes.io/name":      "controller-manager",
					"app.kubernetes.io/component": "controller-manager",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Args: deployArgs},
						},
					},
				},
			},
		}

		authService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "liqo-auth",
				Namespace: liqoNamespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "https",
						Port:     443,
						NodePort: 34000,
					},
				},
				ClusterIP: authEndpoint,
				Type:      "LoadBalancer",
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							IP: authEndpoint,
						},
					},
				},
				Conditions: nil,
			},
		}
		return fakecontroller.NewClientBuilder().WithObjects(ns, secret, clusterIDConfigMap, discoveryDeploy, authService).Build()
	}

	DescribeTable("A generate command is performed",
		func(deployArgs []string, expected string) {
			k8sClient = setUpEnvironment(liqoNamespace, localClusterID, clusterName, token, deployArgs)
			Expect(processGenerateCommand(ctx, k8sClient, liqoNamespace, "liqoctl")).To(BeIdenticalTo(expected))
		},
		Entry("Default authentication service endpoint",
			[]string{fmt.Sprintf("--%v=%v", consts.ClusterNameParameter, clusterName)},
			commandName+" add cluster "+clusterName+" --auth-url https://"+authEndpoint+" --id "+localClusterID+" --token "+token,
		),
		Entry("Overridden authentication service endpoint",
			[]string{
				fmt.Sprintf("--%v=%v", consts.ClusterNameParameter, clusterName),
				fmt.Sprintf("--%v=%v", consts.AuthServiceAddressOverrideParameter, "foo.bar.com"),
				fmt.Sprintf("--%v=%v", consts.AuthServicePortOverrideParameter, "8443"),
			},
			commandName+" add cluster "+clusterName+" --auth-url https://foo.bar.com:8443 --id "+localClusterID+" --token "+token,
		),
	)
})
