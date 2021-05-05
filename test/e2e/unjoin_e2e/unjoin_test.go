package unjoin_e2e

import (
	context2 "context"
	"fmt"
	"testing"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/test/e2e/util"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

func TestUnjoin(t *testing.T) {
	context := util.GetTester()
	NoPods(context.Client1, context.Namespace, t, "cluster1")

	NoJoined(context.Client2, t, "cluster2")
	util.ArePodsUp(context.Client2, context.Namespace, t, "cluster2")
}

func NoPods(clientset *kubernetes.Clientset, namespace string, t *testing.T, clustername string) {
	pods, err := clientset.CoreV1().Pods(namespace).List(context2.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	assert.Equal(t, len(pods.Items), 0, "There are still running pods on "+clustername)
}

func NoJoined(clientset *kubernetes.Clientset, t *testing.T, clustername string) {
	nodes, err := clientset.CoreV1().Nodes().List(context2.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%v=%v", liqoconst.TypeLabel, liqoconst.TypeNode),
	})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	assert.Equal(t, len(nodes.Items), 0, "There are still virtual nodes on "+clustername)
}
