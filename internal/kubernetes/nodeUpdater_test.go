package kubernetes

import (
	v1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	"github.com/liqoTech/liqo/internal/kubernetes/test"
	"github.com/liqoTech/liqo/internal/node"
	"github.com/liqoTech/liqo/pkg/crdClient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"testing"
	"time"
)

func TestNodeUpdater(t *testing.T) {
	// set the client in fake mode
	crdClient.Fake = true

	// create fake client for the home cluster
	client, err := v1.CreateAdvertisementClient("", nil)
	if err != nil {
		t.Fatal(err)
	}

	// instantiate a fake provider
	p := KubernetesProvider{
		Reflector:        &Reflector{started: false},
		nodeUpdateClient: client,
		homeClient:       client,
		nodeName:         test.NodeName,
		startTime:        time.Time{},
		foreignClusterId: test.ForeignClusterId,
		homeClusterID:    test.HomeClusterId,
	}

	var nodeRunner *node.NodeController

	adv := &v1.Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name: test.AdvName,
		},
		Spec: v1.AdvertisementSpec{
			ClusterId:  test.ForeignClusterId,
			Images:     nil,
			LimitRange: corev1.LimitRangeSpec{},
			ResourceQuota: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					"cpu":    test.Cpu1,
					"memory": test.Memory1,
				},
			},
		},
	}

	_, err = client.Client().CoreV1().Nodes().Create(test.NodeTestCases.InputNode)
	if err != nil {
		t.Fatal(err)
	}

	nodeRunner, err = node.NewNodeController(
		node.NaiveNodeProvider{},
		test.NodeTestCases.InputNode,
		client.Client().CoreV1().Nodes())
	if err != nil {
		t.Fatal(err)
	}

	nodeReady, _, err := p.StartNodeUpdater(nodeRunner)
	if err != nil {
		klog.Fatal(err)
	}
	close(nodeReady)

	if _, err := client.Resource("advertisements").Create(adv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	n, err := client.Client().CoreV1().Nodes().Get(test.NodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if !test.AssertNodeCoherency(n, test.NodeTestCases.ExpectedNodes[0]) {
		t.Fatal("node coherency after advertisement creation not asserted")
	} else {
		klog.Info("node coherency after advertisement creation asserted")
	}

	adv.Spec.ResourceQuota.Hard["cpu"] = test.Cpu2
	adv.Spec.ResourceQuota.Hard["memory"] = test.Memory2

	if _, err := client.Resource("advertisements").Update(adv.Name, adv, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	if n, err = client.Client().CoreV1().Nodes().Get(test.NodeName, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}

	if !test.AssertNodeCoherency(n, test.NodeTestCases.ExpectedNodes[1]) {
		t.Fatal("node coherency after advertisement update not asserted")
	} else {
		klog.Info("node coherency after advertisement update asserted")
	}
}
