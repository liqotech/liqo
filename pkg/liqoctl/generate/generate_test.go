// Copyright 2019-2023 The Liqo Authors
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

const (
	commandName      = "liqoctl"
	token            = "local-token"
	authEndpoint     = "1.1.1.1"
	localClusterID   = "local-cluster-id"
	localClusterName = "local-cluster-name"
)

var (
	ctx     context.Context
	options *Options
)

var _ = Describe("Test the generate command works as expected", func() {
	setup := func(deployArgs []string, authServiceAnnotations map[string]string) {
		options = &Options{CommandName: commandName, Factory: &factory.Factory{LiqoNamespace: "liqo-non-standard"}}

		// Create Namespace
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: options.LiqoNamespace}}
		clusterIDConfigMap := testutil.FakeClusterIDConfigMap(options.LiqoNamespace, localClusterID, localClusterName)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: auth.TokenSecretName, Namespace: options.LiqoNamespace},
			Data: map[string][]byte{
				"token": []byte(token),
			},
		}

		discoveryDeploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "whatever", Namespace: options.LiqoNamespace,
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
				Name: "liqo-auth", Namespace: options.LiqoNamespace,
				Annotations: authServiceAnnotations,
			},
			Spec: corev1.ServiceSpec{
				Ports:     []corev1.ServicePort{{Name: "https", Port: 443, NodePort: 34000}},
				ClusterIP: "10.1.0.1",
				Type:      corev1.ServiceTypeLoadBalancer,
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{IP: authEndpoint}},
				},
			},
		}

		options.Factory.CRClient = fake.NewClientBuilder().WithObjects(ns, secret, clusterIDConfigMap, discoveryDeploy, authService).Build()
	}

	DescribeTable("A generate command is performed",
		func(deployArgs []string, authServiceAnnotations map[string]string, expected string) {
			setup(deployArgs, authServiceAnnotations)
			Expect(options.generate(ctx)).To(BeIdenticalTo(expected))
		},
		Entry("Default authentication service endpoint",
			[]string{fmt.Sprintf("--%v=%v", consts.ClusterNameParameter, localClusterName)},
			map[string]string{},
			commandName+" peer out-of-band "+localClusterName+" --auth-url https://"+authEndpoint+" --cluster-id "+localClusterID+" --auth-token "+token,
		),
		Entry("Overridden authentication service endpoint",
			[]string{
				fmt.Sprintf("--%v=%v", consts.ClusterNameParameter, localClusterName),
			},
			map[string]string{
				consts.OverrideAddressAnnotation: "foo.bar.com",
				consts.OverridePortAnnotation:    "8443",
			},
			commandName+" peer out-of-band "+localClusterName+" --auth-url https://foo.bar.com:8443 --cluster-id "+localClusterID+" --auth-token "+token,
		),
	)
})
