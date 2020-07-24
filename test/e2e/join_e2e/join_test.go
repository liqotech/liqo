package join_e2e

import (
	context2 "context"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sync"
	"testing"
)
import "os"

type Tester struct {
	client1   *kubernetes.Clientset
	client2   *kubernetes.Clientset
	namespace string
}

var l = &sync.Mutex{}

var (
	t1 *Tester
)

func GetTester() *Tester {
	l.Lock()
	defer l.Unlock()

	if t1 == nil {
		t1 = createTester()
	}

	return t1
}

func createTester() *Tester {
	kubeconfig1 := os.Getenv("KUBECONFIG_1")
	if kubeconfig1 == "" {
		klog.Error("KUBECONFIG_1 not set")
		os.Exit(1)
	}
	kubeconfig2 := os.Getenv("KUBECONFIG_2")
	if kubeconfig2 == "" {
		klog.Error("KUBECONFIG_2 not set")
		os.Exit(1)
	}
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		klog.Error("NAMESPACE not set")
		os.Exit(1)
	}

	config1, err := clientcmd.BuildConfigFromFlags("", kubeconfig1)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	config2, err := clientcmd.BuildConfigFromFlags("", kubeconfig2)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	clientset1, err := kubernetes.NewForConfig(config1)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	clientset2, err := kubernetes.NewForConfig(config2)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	return &Tester{
		client1:   clientset1,
		client2:   clientset2,
		namespace: namespace,
	}
}

func TestPodsUp1(t *testing.T) {
	context := GetTester()
	ArePodsUp(context.client1, context.namespace, t, "cluster1")
}

func TestPodsUp2(t *testing.T) {
	context := GetTester()
	ArePodsUp(context.client2, context.namespace, t, "cluster2")
}

func TestNodeVK1(t *testing.T) {
	context := GetTester()
	CheckVkNode(context.client1, context.client2, context.namespace, t)
}

func TestNodeVK2(t *testing.T) {
	context := GetTester()
	CheckVkNode(context.client2, context.client1, context.namespace, t)
}

func ArePodsUp(clientset *kubernetes.Clientset, namespace string, t *testing.T, clustername string) {
	pods, err := clientset.CoreV1().Pods(namespace).List(context2.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	for _, num := range pods.Items {
		for _, container := range num.Status.ContainerStatuses {
			assert.Equal(t, true, container.Ready, "Asserting "+container.Name+"pods is running "+
				"on "+clustername)
		}
	}
}

func CheckVkNode(client1 *kubernetes.Clientset, client2 *kubernetes.Clientset, namespace string, t *testing.T) {
	id, err := client1.CoreV1().ConfigMaps(namespace).Get(context2.TODO(), "cluster-id", metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	node, err := client2.CoreV1().Nodes().Get(context2.TODO(), "vk-"+id.Data["cluster-id"], metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	for _, condition := range node.Status.Conditions {
		if condition.Type == "Ready" {
			assert.Equal(t, "True", string(condition.Status), "Assert the node"+node.Name+"is ready")
		} else {
			assert.Equal(t, "False", string(condition.Status), "Assert the other node conditions on node "+node.Name+"are not True")
		}
	}
}
