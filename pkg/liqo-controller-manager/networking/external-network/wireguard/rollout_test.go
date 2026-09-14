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

package wireguard

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/forge"
)

func newTestClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	Expect(appsv1.AddToScheme(scheme)).To(Succeed())
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func newIntegrationClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	Expect(networkingv1beta1.AddToScheme(scheme)).To(Succeed())
	Expect(appsv1.AddToScheme(scheme)).To(Succeed())
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(rbacv1.AddToScheme(scheme)).To(Succeed())
	Expect(monitoringv1.AddToScheme(scheme)).To(Succeed())
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&networkingv1beta1.WgGatewayServer{}, &appsv1.Deployment{}).
		Build()
}

func makeDeployment(name, remoteClusterID, templateName, templateNamespace, generation string, rolling bool) *appsv1.Deployment {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  "ns",
			Generation: 1,
			Labels: map[string]string{
				consts.RemoteClusterID:        remoteClusterID,
				consts.NetworkingComponentKey: gateway.GatewayComponentGateway,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "gateway", Image: "liqo/gateway:latest"},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration:  1,
			Replicas:            1,
			UpdatedReplicas:     1,
			AvailableReplicas:   1,
			UnavailableReplicas: 0,
		},
	}
	if templateName != "" || templateNamespace != "" || generation != "" {
		if dep.Annotations == nil {
			dep.Annotations = map[string]string{}
		}
		if templateName != "" {
			dep.Annotations[consts.TemplateNameAnnotationKey] = templateName
		}
		if templateNamespace != "" {
			dep.Annotations[consts.TemplateNamespaceAnnotationKey] = templateNamespace
		}
		if generation != "" {
			dep.Annotations[consts.TemplateGenerationAnnotationKey] = generation
		}
	}
	if rolling {
		dep.Status.ObservedGeneration = 0
	}
	return dep
}

func buildWgGatewayServer(name, namespace, remoteClusterID string) *networkingv1beta1.WgGatewayServer {
	return &networkingv1beta1.WgGatewayServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(name + "-uid"),
			Labels: map[string]string{
				consts.RemoteClusterID: remoteClusterID,
			},
			Annotations: map[string]string{
				consts.TemplateNameAnnotationKey:       "server-template",
				consts.TemplateNamespaceAnnotationKey:  "server-template-ns",
				consts.TemplateGenerationAnnotationKey: "2",
			},
			Finalizers: []string{consts.ClusterRoleBindingFinalizer},
		},
		Spec: networkingv1beta1.WgGatewayServerSpec{
			SecretRef: corev1.LocalObjectReference{
				Name: name + "-keys",
			},
			Service: networkingv1beta1.ServiceTemplate{
				Metadata: metav1.ObjectMeta{
					Labels: map[string]string{
						consts.RemoteClusterID:        remoteClusterID,
						consts.NetworkingComponentKey: gateway.GatewayComponentGateway,
					},
				},
				Spec: corev1.ServiceSpec{
					Type:       corev1.ServiceTypeClusterIP,
					ClusterIP:  "10.0.0.1",
					ClusterIPs: []string{"10.0.0.1"},
					Ports: []corev1.ServicePort{
						{Port: 51820, Protocol: corev1.ProtocolUDP},
					},
				},
			},
			Deployment: networkingv1beta1.DeploymentTemplate{
				Metadata: metav1.ObjectMeta{
					Labels: map[string]string{
						consts.RemoteClusterID:        remoteClusterID,
						consts.NetworkingComponentKey: gateway.GatewayComponentGateway,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": forge.GatewayResourceName(name),
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": forge.GatewayResourceName(name),
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "wireguard",
									Image: "liqo/gateway-wireguard:latest",
									Env: []corev1.EnvVar{
										{Name: "VERSION", Value: "new"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func buildExistingDeployment(wgServer *networkingv1beta1.WgGatewayServer) *appsv1.Deployment {
	desired := wgServer.Spec.Deployment.Spec.DeepCopy()
	desired.Template.Spec.Containers[0].Env[0].Value = "old"
	depName := forge.GatewayResourceName(wgServer.Name)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       depName,
			Namespace:  wgServer.Namespace,
			Generation: 1,
			Labels: map[string]string{
				consts.RemoteClusterID:        wgServer.Labels[consts.RemoteClusterID],
				consts.NetworkingComponentKey: gateway.GatewayComponentGateway,
			},
			Annotations: map[string]string{
				consts.TemplateNameAnnotationKey:       wgServer.Annotations[consts.TemplateNameAnnotationKey],
				consts.TemplateNamespaceAnnotationKey:  wgServer.Annotations[consts.TemplateNamespaceAnnotationKey],
				consts.TemplateGenerationAnnotationKey: "1",
			},
		},
		Spec: *desired,
		Status: appsv1.DeploymentStatus{
			ObservedGeneration:  1,
			Replicas:            1,
			UpdatedReplicas:     1,
			AvailableReplicas:   1,
			UnavailableReplicas: 0,
		},
	}
}

func buildService(wgServer *networkingv1beta1.WgGatewayServer) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      forge.GatewayResourceName(wgServer.Name),
			Namespace: wgServer.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:       corev1.ServiceTypeClusterIP,
			ClusterIP:  "10.0.0.1",
			ClusterIPs: []string{"10.0.0.1"},
			Ports: []corev1.ServicePort{
				{Port: 51820, Protocol: corev1.ProtocolUDP},
			},
		},
	}
}

func buildActivePod(wgServer *networkingv1beta1.WgGatewayServer) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      forge.GatewayResourceName(wgServer.Name) + "-abc",
			Namespace: wgServer.Namespace,
			Labels: map[string]string{
				"app": forge.GatewayResourceName(wgServer.Name),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "wireguard", Image: "liqo/gateway-wireguard:latest"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.1.10",
		},
	}
}

func buildSecret(wgServer *networkingv1beta1.WgGatewayServer) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wgServer.Spec.SecretRef.Name,
			Namespace: wgServer.Namespace,
			Labels: map[string]string{
				consts.RemoteClusterID:        wgServer.Labels[consts.RemoteClusterID],
				consts.GatewayResourceLabel:   consts.GatewayResourceLabelValue,
				consts.NetworkingComponentKey: gateway.GatewayComponentGateway,
			},
		},
		Data: map[string][]byte{
			consts.PrivateKeyField: []byte("private-key"),
			consts.PublicKeyField:  []byte("public-key"),
		},
	}
}

var _ = Describe("Rollout helpers", func() {
	Context("IsDeploymentRollingOut", func() {
		DescribeTable("reports whether a Deployment is rolling out",
			func(dep *appsv1.Deployment, expected bool) {
				Expect(IsDeploymentRollingOut(dep)).To(Equal(expected))
			},
			Entry("nil deployment", nil, false),
			Entry("steady state", makeDeployment("gw", "remote", "template", "template-ns", "generation", false), false),
			Entry("unobserved generation", func() *appsv1.Deployment {
				d := makeDeployment("gw", "remote", "template", "template-ns", "generation", false)
				d.Generation = 2
				d.Status.ObservedGeneration = 1
				return d
			}(), true),
			Entry("updated replicas below desired", func() *appsv1.Deployment {
				d := makeDeployment("gw", "remote", "template", "template-ns", "generation", false)
				d.Status.UpdatedReplicas = 0
				return d
			}(), true),
			Entry("available replicas below desired", func() *appsv1.Deployment {
				d := makeDeployment("gw", "remote", "template", "template-ns", "generation", false)
				d.Status.AvailableReplicas = 0
				return d
			}(), true),
			Entry("unavailable replicas present", func() *appsv1.Deployment {
				d := makeDeployment("gw", "remote", "template", "template-ns", "generation", false)
				d.Status.UnavailableReplicas = 1
				return d
			}(), true),
		)
	})

	Context("ShouldDelayRollout", func() {
		const (
			remoteClusterID          = "remote"
			desiredTemplateName      = "template"
			desiredTemplateNamespace = "template-ns"
			desiredGeneration        = "desired-generation"
		)

		DescribeTable("decides whether a rollout should be delayed",
			func(self types.NamespacedName, peers []*appsv1.Deployment, expected bool) {
				objs := make([]client.Object, len(peers))
				for i := range peers {
					objs[i] = peers[i]
				}
				cl := newTestClient(objs...)
				delay, _, err := ShouldDelayRollout(context.Background(), cl, remoteClusterID, self,
					desiredTemplateName, desiredTemplateNamespace, desiredGeneration)
				Expect(err).NotTo(HaveOccurred())
				Expect(delay).To(Equal(expected))
			},
			Entry("no peers", types.NamespacedName{Namespace: "ns", Name: "gw-a"}, nil, false),
			Entry("self is excluded from peer list",
				types.NamespacedName{Namespace: "ns", Name: "gw-a"},
				[]*appsv1.Deployment{makeDeployment("gw-a", remoteClusterID, desiredTemplateName, desiredTemplateNamespace, desiredGeneration, false)},
				false,
			),
			Entry("lower-named peer with stale generation delays",
				types.NamespacedName{Namespace: "ns", Name: "gw-b"},
				[]*appsv1.Deployment{makeDeployment("gw-a", remoteClusterID, desiredTemplateName, desiredTemplateNamespace, "old-generation", false)},
				true,
			),
			Entry("lower-named peer with desired generation but rolling out delays",
				types.NamespacedName{Namespace: "ns", Name: "gw-b"},
				[]*appsv1.Deployment{makeDeployment("gw-a", remoteClusterID, desiredTemplateName, desiredTemplateNamespace, desiredGeneration, true)},
				true,
			),
			Entry("lower-named peer up to date does not delay",
				types.NamespacedName{Namespace: "ns", Name: "gw-b"},
				[]*appsv1.Deployment{makeDeployment("gw-a", remoteClusterID, desiredTemplateName, desiredTemplateNamespace, desiredGeneration, false)},
				false,
			),
			Entry("higher-named peer with stale generation does not delay",
				types.NamespacedName{Namespace: "ns", Name: "gw-a"},
				[]*appsv1.Deployment{makeDeployment("gw-b", remoteClusterID, desiredTemplateName, desiredTemplateNamespace, "old-generation", false)},
				false,
			),
			Entry("higher-named peer rolling out delays",
				types.NamespacedName{Namespace: "ns", Name: "gw-a"},
				[]*appsv1.Deployment{makeDeployment("gw-b", remoteClusterID, desiredTemplateName, desiredTemplateNamespace, desiredGeneration, true)},
				true,
			),
			Entry("peer with different remote cluster id is ignored",
				types.NamespacedName{Namespace: "ns", Name: "gw-a"},
				[]*appsv1.Deployment{makeDeployment("gw-b", "other-cluster", desiredTemplateName, desiredTemplateNamespace, "old-generation", true)},
				false,
			),
			Entry("peer with same remote cluster id but different template name is ignored",
				types.NamespacedName{Namespace: "ns", Name: "gw-b"},
				[]*appsv1.Deployment{makeDeployment("gw-a", remoteClusterID, "other-template", desiredTemplateNamespace, "old-generation", true)},
				false,
			),
			Entry("peer with same remote cluster id and name but different template namespace is ignored",
				types.NamespacedName{Namespace: "ns", Name: "gw-b"},
				[]*appsv1.Deployment{makeDeployment("gw-a", remoteClusterID, desiredTemplateName, "other-namespace", "old-generation", true)},
				false,
			),
		)
	})
})

var _ = Describe("WgGatewayServer controller serialization gate", Ordered, func() {
	const remoteClusterID = "remote"

	var (
		ctx      context.Context
		cl       client.Client
		r        *WgGatewayServerReconciler
		wgA      *networkingv1beta1.WgGatewayServer
		wgB      *networkingv1beta1.WgGatewayServer
		depA     *appsv1.Deployment
		depB     *appsv1.Deployment
		updatedA appsv1.Deployment
	)

	BeforeAll(func() {
		ctx = context.Background()

		wgA = buildWgGatewayServer("a", "test-a", remoteClusterID)
		wgB = buildWgGatewayServer("b", "test-b", remoteClusterID)
		depA = buildExistingDeployment(wgA)
		depB = buildExistingDeployment(wgB)

		cl = newIntegrationClient(
			wgA, wgB,
			depA, depB,
			buildService(wgA), buildService(wgB),
			buildActivePod(wgA), buildActivePod(wgB),
			buildSecret(wgA), buildSecret(wgB),
		)

		recorder := record.NewFakeRecorder(100)
		r = NewWgGatewayServerReconciler(cl, cl.Scheme(), recorder, "liqo-gateway")
	})

	It("allows the lowest-named peer to proceed and update its Deployment", func() {
		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: wgA.Namespace, Name: wgA.Name}})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.RequeueAfter).To(BeZero())

		Expect(cl.Get(ctx, client.ObjectKeyFromObject(depA), &updatedA)).To(Succeed())
		Expect(updatedA.Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal("new"))
		Expect(updatedA.Annotations).To(HaveKey(consts.TemplateNameAnnotationKey))
		Expect(updatedA.Annotations[consts.TemplateNameAnnotationKey]).To(Equal("server-template"))
		Expect(updatedA.Annotations).To(HaveKey(consts.TemplateNamespaceAnnotationKey))
		Expect(updatedA.Annotations[consts.TemplateNamespaceAnnotationKey]).To(Equal("server-template-ns"))
		Expect(updatedA.Annotations).To(HaveKey(consts.TemplateGenerationAnnotationKey))
		Expect(updatedA.Annotations[consts.TemplateGenerationAnnotationKey]).To(Equal("2"))
	})

	It("delays the higher-named peer while the lower-named peer is rolling out", func() {
		updatedA.Generation = 2
		Expect(cl.Update(ctx, &updatedA)).To(Succeed())

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: wgB.Namespace, Name: wgB.Name}})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.RequeueAfter).To(BeNumerically(">", 0))

		var unchangedB appsv1.Deployment
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(depB), &unchangedB)).To(Succeed())
		Expect(unchangedB.Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal("old"))
	})

	It("allows the higher-named peer to roll after the lower-named peer finishes", func() {
		updatedA.Status.ObservedGeneration = updatedA.Generation
		updatedA.Status.UpdatedReplicas = 1
		updatedA.Status.AvailableReplicas = 1
		updatedA.Status.UnavailableReplicas = 0
		Expect(cl.Status().Update(ctx, &updatedA)).To(Succeed())

		res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: wgB.Namespace, Name: wgB.Name}})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.RequeueAfter).To(BeZero())

		var updatedB appsv1.Deployment
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(depB), &updatedB)).To(Succeed())
		Expect(updatedB.Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal("new"))
	})
})
