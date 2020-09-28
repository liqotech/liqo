package e2e

import (
	context2 "context"
	"github.com/liqotech/liqo/test/e2e/util"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"testing"
)

func testJoin(t *testing.T) {
	t.Run("testPodsUp1", testPodsUp1)
	t.Run("testPodsUp2", testPodsUp2)
	t.Run("testNodeVK1", testNodeVK1)
	t.Run("testNodeVK2", testNodeVK2)
}

func testPodsUp1(t *testing.T) {
	context := util.GetTester()
	util.ArePodsUp(context.Client1, context.Namespace, t, "cluster1")
}

func testPodsUp2(t *testing.T) {
	context := util.GetTester()
	util.ArePodsUp(context.Client2, context.Namespace, t, "cluster2")
}

func testNodeVK1(t *testing.T) {
	context := util.GetTester()
	CheckVkNode(context.Client1, context.Client2, context.Namespace, t)
}

func testNodeVK2(t *testing.T) {
	context := util.GetTester()
	CheckVkNode(context.Client2, context.Client1, context.Namespace, t)
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
