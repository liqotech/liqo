package join_e2e

import (
	context2 "context"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"testing"
	"github.com/liqoTech/liqo/test/e2e/util"
)


func TestPodsUp1(t *testing.T) {
	context := util.GetTester()
	ArePodsUp(context.Client1, context.Namespace, t, "cluster1")
}

func TestPodsUp2(t *testing.T) {
	context := util.GetTester()
	ArePodsUp(context.Client2, context.Namespace, t, "cluster2")
}

func TestNodeVK1(t *testing.T) {
	context := util.GetTester()
	CheckVkNode(context.Client1, context.Client2, context.Namespace, t)
}

func TestNodeVK2(t *testing.T) {
	context := util.GetTester()
	CheckVkNode(context.Client2, context.Client1, context.Namespace, t)
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
	node, err := client2.CoreV1().Nodes().Get(context2.TODO(), "liqo-"+id.Data["cluster-id"], metav1.GetOptions{})
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
