package kubernetes

import (
	"context"
	v1 "github.com/liqotech/liqo/api/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/api/sharing/v1alpha1"
	"github.com/liqotech/liqo/internal/kubernetes/test"
	"github.com/liqotech/liqo/internal/node"
	"github.com/liqotech/liqo/pkg/crdClient"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"testing"
	"time"
)

func TestNodeUpdater(t *testing.T) {
	// set the client in fake mode
	crdClient.Fake = true

	// create fake client for the home cluster
	advClient, err := advtypes.CreateAdvertisementClient("", nil)
	if err != nil {
		t.Fatal(err)
	}

	tepClient, err := v1.CreateTunnelEndpointClient("")
	if err != nil {
		t.Fatal(err)
	}

	// instantiate a fake provider
	p := KubernetesProvider{
		Reflector:        &Reflector{started: false},
		advClient:        advClient,
		homeClient:       advClient,
		tunEndClient:     tepClient,
		nodeName:         test.NodeName,
		startTime:        time.Time{},
		foreignClusterId: test.ForeignClusterId,
		homeClusterID:    test.HomeClusterId,
	}

	var nodeRunner *node.NodeController

	adv := &advtypes.Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name: test.AdvName,
		},
		Spec: advtypes.AdvertisementSpec{
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

	_, err = advClient.Client().CoreV1().Nodes().Create(context.TODO(), test.NodeTestCases.InputNode, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	nodeRunner, err = node.NewNodeController(
		node.NaiveNodeProvider{},
		test.NodeTestCases.InputNode,
		advClient.Client().CoreV1().Nodes())
	if err != nil {
		t.Fatal(err)
	}

	nodeReady, _, err := p.StartNodeUpdater(nodeRunner)
	if err != nil {
		klog.Fatal(err)
	}
	close(nodeReady)

	if _, err := advClient.Resource("advertisements").Create(adv, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	n, err := advClient.Client().CoreV1().Nodes().Get(context.TODO(), test.NodeName, metav1.GetOptions{})
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

	if _, err := advClient.Resource("advertisements").Update(adv.Name, adv, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	if n, err = advClient.Client().CoreV1().Nodes().Get(context.TODO(), test.NodeName, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}

	if !test.AssertNodeCoherency(n, test.NodeTestCases.ExpectedNodes[1]) {
		t.Fatal("node coherency after advertisement update not asserted")
	} else {
		klog.Info("node coherency after advertisement update asserted")
	}

	// test network
	// create a TunnelEndpoint: this will trigger update of node status to Ready
	tep := &v1.TunnelEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name: test.TepName,
		},
		Spec: v1.TunnelEndpointSpec{
			ClusterID:      test.ForeignClusterId,
			PodCIDR:        test.PodCIDR,
			TunnelPublicIP: test.TunnelPublicIP,
		},
	}

	if _, err := p.tunEndClient.Resource("tunnelendpoints").Create(tep, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	if n, err = advClient.Client().CoreV1().Nodes().Get(context.TODO(), test.NodeName, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}

	if !test.AssertNodeCoherency(n, test.NodeTestCases.ExpectedNodes[2]) {
		t.Fatal("node coherency after tunnelEndpoint update not asserted")
	} else {
		klog.Info("node coherency after tunnelEndpoint update asserted")
	}

	assert.Equal(t, test.PodCIDR, p.RemoteRemappedPodCidr)
	assert.Equal(t, "", p.LocalRemappedPodCidr)

	// delete the TunnelEndpoint: the node should become NotReady
	if err := p.tunEndClient.Resource("tunnelendpoints").Delete(tep.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	if n, err = advClient.Client().CoreV1().Nodes().Get(context.TODO(), test.NodeName, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}

	if !test.AssertNodeCoherency(n, test.NodeTestCases.ExpectedNodes[1]) {
		t.Fatal("node coherency after tunnelEndpoint update not asserted")
	} else {
		klog.Info("node coherency after tunnelEndpoint update asserted")
	}

	assert.Equal(t, "", p.RemoteRemappedPodCidr)

	// create a TunnelEndpoint with network remapping: the node should become Ready
	tep = &v1.TunnelEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name: test.TepName,
		},
		Spec: v1.TunnelEndpointSpec{
			ClusterID:      test.ForeignClusterId,
			PodCIDR:        test.PodCIDR,
			TunnelPublicIP: test.TunnelPublicIP,
		},
		Status: v1.TunnelEndpointStatus{
			LocalRemappedPodCIDR:  test.LocalRemappedPodCIDR,
			RemoteRemappedPodCIDR: test.RemoteRemappedPodCIDR,
		},
	}

	if _, err := p.tunEndClient.Resource("tunnelendpoints").Create(tep, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	if n, err = advClient.Client().CoreV1().Nodes().Get(context.TODO(), test.NodeName, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}

	if !test.AssertNodeCoherency(n, test.NodeTestCases.ExpectedNodes[2]) {
		t.Fatal("node coherency after tunnelEndpoint update not asserted")
	} else {
		klog.Info("node coherency after tunnelEndpoint update asserted")
	}

	assert.Equal(t, test.RemoteRemappedPodCIDR, p.RemoteRemappedPodCidr)
	assert.Equal(t, test.LocalRemappedPodCIDR, p.LocalRemappedPodCidr)

	// test unjoin
	// set advertisement status to DELETING
	adv.Status.AdvertisementStatus = advtypes.AdvertisementDeleting
	if _, err := advClient.Resource("advertisements").UpdateStatus(adv.Name, adv, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)

	// the node should go in NotReady status
	if n, err = advClient.Client().CoreV1().Nodes().Get(context.TODO(), test.NodeName, metav1.GetOptions{}); err != nil {
		t.Fatal(err)
	}
	for i, condition := range n.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			assert.Equal(t, corev1.ConditionFalse, n.Status.Conditions[i].Status)
			break
		}
	}

	// the adv should have been deleted
	_, err = advClient.Resource("advertisements").Get(adv.Name, metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))
}
