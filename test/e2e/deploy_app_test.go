package e2e

import (
	"context"
	"fmt"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"testing"
	"time"

	http_helper "github.com/gruntwork-io/terratest/modules/http-helper"
	"github.com/gruntwork-io/terratest/modules/k8s"
	liqoControllerManager "github.com/liqotech/liqo/pkg/liqo-controller-manager"
)

const (
	retries             = 36
	sleepBetweenRetries = 5 * time.Second
)

func testDeployApp(t *testing.T) {
	kubeResourcePath := "https://raw.githubusercontent.com/liqotech/microservices-demo/master/release/kubernetes-manifests.yaml"

	namespace := "test-app"
	configPath, ok := os.LookupEnv("KUBECONFIG_1")
	assert.Assert(t, ok)
	options := k8s.NewKubectlOptions("", configPath, namespace)

	defer cleanup(t, options, kubeResourcePath, namespace)

	k8s.CreateNamespace(t, options, namespace)
	liqoEnable(t, options, namespace)
	k8s.KubectlApply(t, options, kubeResourcePath)

	// load generator pods is in error state
	pods := k8s.ListPods(t, options, metav1.ListOptions{
		LabelSelector: "app!=loadgenerator",
	})
	for _, pod := range pods {
		k8s.WaitUntilPodAvailable(t, options, pod.Name, retries, sleepBetweenRetries)
	}

	svcs := k8s.ListServices(t, options, metav1.ListOptions{})
	for _, svc := range svcs {
		// load balancer services will be never available in kind
		if svc.Spec.Type != v1.ServiceTypeLoadBalancer {
			k8s.WaitUntilServiceAvailable(t, options, svc.Name, retries, sleepBetweenRetries)
		}
	}

	service := k8s.GetService(t, options, "frontend-external")
	assert.Assert(t, len(service.Spec.Ports) > 0)

	nodes := getNodes(t, options)
	assert.Assert(t, len(nodes) > 0)

	url := fmt.Sprintf("http://%s:%d", getAddress(t, nodes[0].Status.Addresses), service.Spec.Ports[0].NodePort)
	http_helper.HttpGetWithRetryWithCustomValidation(t, url, nil, retries, sleepBetweenRetries, func(code int, body string) bool {
		return code == 200
	})
}

func getAddress(t *testing.T, addrs []v1.NodeAddress) string {
	for _, addr := range addrs {
		if addr.Type == v1.NodeInternalIP {
			return addr.Address
		}
	}
	t.Fail()
	return ""
}

func cleanup(t *testing.T, options *k8s.KubectlOptions, configPath string, namespace string) {
	k8s.KubectlDelete(t, options, configPath)
	clientset, err := k8s.GetKubernetesClientFromOptionsE(t, options)
	assert.NilError(t, err)
	for {
		pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
		assert.NilError(t, err)
		if len(pods.Items) == 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	k8s.DeleteNamespace(t, options, namespace)
}

func getNodes(t *testing.T, options *k8s.KubectlOptions) []v1.Node {
	clientset, err := k8s.GetKubernetesClientFromOptionsE(t, options)
	assert.NilError(t, err)

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%v!=%v,!net.liqo.io/gateway", liqoControllerManager.TypeLabel, liqoControllerManager.TypeNode),
	})
	assert.NilError(t, err)
	return nodes.Items
}

func liqoEnable(t *testing.T, options *k8s.KubectlOptions, namespace string) {
	clientset, err := k8s.GetKubernetesClientFromOptionsE(t, options)
	assert.NilError(t, err)

	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
	assert.NilError(t, err)

	if ns.Labels == nil {
		ns.Labels = map[string]string{}
	}
	ns.Labels["liqo.io/enabled"] = "true"

	_, err = clientset.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
	assert.NilError(t, err)
}
