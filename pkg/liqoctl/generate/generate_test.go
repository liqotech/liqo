package generate

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakecontroller "sigs.k8s.io/controller-runtime/pkg/client/fake"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
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
	k8sClient       client.Client
	ctx             context.Context
	expectedCommand string
)

func TestAddCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Parameters Fetching")
}

var _ = Describe("Test the generate command works as expected", func() {

	When("A generate command is performed", func() {
		BeforeEach(func() {
			k8sClient = setUpEnvironment(liqoNamespace, localClusterID, token, clusterName)
			expectedCommand = commandName + " add cluster " + clusterName + " --auth-url https://" + authEndpoint +
				" --id " + localClusterID + " --token " + token
		})

		It("Should be equal to the expected output", func() {
			command := processGenerateCommand(ctx, k8sClient, liqoNamespace, "liqoctl")
			Expect(command).To(BeIdenticalTo(expectedCommand))
		})
	})
})

func setUpEnvironment(liqonamespace, localClusterID, token, clusterName string) client.Client {
	// Create Namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: liqonamespace,
		},
	}
	// Create ClusterID ConfigMap
	clusterIDConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consts.ClusterIDConfigMapName,
			Namespace: liqonamespace,
		},
		Data: map[string]string{
			consts.ClusterIDConfigMapKey: localClusterID,
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
	clusterConfig := &configv1alpha1.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "liqo-configuration",
		},
		Spec: configv1alpha1.ClusterConfigSpec{

			DiscoveryConfig: configv1alpha1.DiscoveryConfig{
				ClusterName: clusterName,
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
	return fakecontroller.NewClientBuilder().WithObjects(ns, secret, clusterIDConfigMap, clusterConfig, authService).Build()
}
