package microservices

import (
	"context"
	"fmt"
	"time"

	http_helper "github.com/gruntwork-io/terratest/modules/http-helper"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/test/e2e/testutils"
)

const (
	retries             = 120
	sleepBetweenRetries = 3 * time.Second
	// TestNamespaceName is the namespace name where the test is performed.
	TestNamespaceName = "test-app"
)

// DeployApp creates the namespace and deploy the applications. It returns an error in case of failures.
func DeployApp(t ginkgo.GinkgoTInterface, configPath, kubeResourcePath string) error {
	options := k8s.NewKubectlOptions("", configPath, TestNamespaceName)
	if err := k8s.CreateNamespaceWithMetadataE(t, options, metav1.ObjectMeta{
		Name:   "test-app",
		Labels: testutils.LiqoTestNamespaceLabels,
	}); err != nil {
		return err
	}
	if err := k8s.KubectlApplyE(t, options, kubeResourcePath); err != nil {
		return err
	}
	return nil
}

// WaitDemoApp waits until each service of the application has ready endpoints. It fails if this does not happen
// within the timeout (retries*sleepBetweenRetries).
func WaitDemoApp(t ginkgo.GinkgoTInterface, options *k8s.KubectlOptions) {
	pods := k8s.ListPods(t, options, metav1.ListOptions{})
	for index := range pods {
		k8s.WaitUntilPodAvailable(t, options, pods[index].Name, retries, sleepBetweenRetries)
	}

	svcs := k8s.ListServices(t, options, metav1.ListOptions{})
	for index := range svcs {
		// load balancer services will be never available in kind
		if svcs[index].Spec.Type != corev1.ServiceTypeLoadBalancer {
			k8s.WaitUntilServiceAvailable(t, options, svcs[index].Name, retries, sleepBetweenRetries)
		}
	}
}

// CheckApplicationIsWorking performs HTTP requests to the micro-service application to assess its functionality and availability.
func CheckApplicationIsWorking(t ginkgo.GinkgoTInterface, options *k8s.KubectlOptions) error {
	service := k8s.GetService(t, options, "frontend-external")
	if len(service.Spec.Ports) == 0 {
		return fmt.Errorf("frontend service not found")
	}

	nodes, err := getNodes(t, options)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes retrieved from the cluster")
	}
	nodeAddress, err := getInternalAddress(nodes[0].Status.Addresses)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://%s:%d", nodeAddress, service.Spec.Ports[0].NodePort)
	return http_helper.HttpGetWithRetryWithCustomValidationE(t, url, nil, retries, sleepBetweenRetries, func(code int, body string) bool {
		return code == 200
	})
}

func getInternalAddress(addrs []corev1.NodeAddress) (string, error) {
	for _, addr := range addrs {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address, nil
		}
	}
	return "", fmt.Errorf("unbale to retrieve an internalIP for the selected node")
}

func getNodes(t ginkgo.GinkgoTInterface, options *k8s.KubectlOptions) ([]corev1.Node, error) {
	clientset, err := k8s.GetKubernetesClientFromOptionsE(t, options)
	if err != nil {
		return nil, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%v!=%v", liqoconst.TypeLabel, liqoconst.TypeNode),
	})
	if err != nil {
		return nil, err
	}

	return nodes.Items, err
}

// CheckPodsNodeAffinity checks if the pods deployed in the namespace are correctly mutated by the webhook.
func CheckPodsNodeAffinity(ctx context.Context, homeClient kubernetes.Interface) bool {
	labelAppKey := "app"
	labelAppValue := "frontend"
	pods, err := homeClient.CoreV1().Pods(TestNamespaceName).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("%s -> unable to list pods in the namespace '%s'", err, TestNamespaceName)
		return false
	}
	if len(pods.Items) == 0 {
		return false
	}
	for i := range pods.Items {
		ginkgo.By(fmt.Sprintf("Checking that pod '%s' has the right node affinity", pods.Items[i].Name))
		if value, ok := pods.Items[i].Labels[labelAppKey]; ok && value == labelAppValue {
			gomega.Expect(*pods.Items[i].Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution).
				To(gomega.Equal(getFrontendPodNodeAffinity()))
			continue
		}
		gomega.Expect(*pods.Items[i].Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution).
			To(gomega.Equal(getDefaultPodNodeAffinity()))
	}
	return true
}

// getDefaultPodNodeAffinity provides the node affinity placed on the pod by the webhook.
func getDefaultPodNodeAffinity() corev1.NodeSelector {
	return corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
		{
			MatchExpressions: []corev1.NodeSelectorRequirement{
				{
					Key:      liqoconst.TypeLabel,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{liqoconst.TypeNode},
				},
			},
		},
		{
			MatchExpressions: []corev1.NodeSelectorRequirement{
				{
					Key:      liqoconst.TypeLabel,
					Operator: corev1.NodeSelectorOpNotIn,
					Values:   []string{liqoconst.TypeNode},
				},
			},
		},
	}}
}

// getFrontendPodNodeAffinity provides the node affinity placed on the frontend pod by the webhook.
func getFrontendPodNodeAffinity() corev1.NodeSelector {
	return corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
		{
			MatchExpressions: []corev1.NodeSelectorRequirement{
				{
					Key:      liqoconst.TypeLabel,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{liqoconst.TypeNode},
				},
				{
					Key:      liqoconst.TypeLabel,
					Operator: corev1.NodeSelectorOpNotIn,
					Values:   []string{liqoconst.TypeNode},
				},
			},
		},
		{
			MatchExpressions: []corev1.NodeSelectorRequirement{
				{
					Key:      liqoconst.TypeLabel,
					Operator: corev1.NodeSelectorOpNotIn,
					Values:   []string{liqoconst.TypeNode},
				},
				{
					Key:      liqoconst.TypeLabel,
					Operator: corev1.NodeSelectorOpNotIn,
					Values:   []string{liqoconst.TypeNode},
				},
			},
		},
	}}
}
