package e2e

import (
	"bytes"
	"context"
	"fmt"
	liqocontrollerutils "github.com/liqotech/liqo/pkg/utils"

	"github.com/liqotech/liqo/pkg/virtualKubelet"
	"github.com/liqotech/liqo/test/e2e/util"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

var (
	image              = "nginx"
	waitTime           = 2 * time.Minute
	podTesterLocalCl1  = "tester-local-cl1"
	podTesterRemoteCl1 = "tester-remote-cl1"
	podTesterLocalCl2  = "tester-local-cl2"
	podTesterRemoteCl2 = "tester-remote-cl2"
	namespaceNameCl1   = "test-connectivity-cl1"
	namespaceNameCl2   = "test-connectivity-cl2"
	//label to list only the real nodes excluding the virtual ones
	labelSelectorNodes = fmt.Sprintf("%v!=%v", liqocontrollerutils.TypeLabel, liqocontrollerutils.TypeNode)
	//TODO: use the retry mechanism of curl without sleeping before running the command
	command = "curl -s -o /dev/null -w '%{http_code}' "
)

func testNet(t *testing.T) {
	t.Run("testPodConnectivity1to2", testPodConnectivity1to2)
	t.Run("testPodConnectivity2to1", testPodConnectivity2to1)
}

func testPodConnectivity1to2(t *testing.T) {
	context := util.GetTester()
	ConnectivityCheckPodToPodCluster1ToCluster2(context, t)
	ConnectivityCheckNodeToPodCluster1ToCluster2(context, t)
	err := util.DeleteNamespace(context.Client1, namespaceNameCl1)
	assert.Nil(t, err, "error should be nil while deleting namespace %s in cluster %s", namespaceNameCl1, context.ClusterID1)

}

func testPodConnectivity2to1(t *testing.T) {
	context := util.GetTester()
	ConnectivityCheckPodToPodCluster2ToCluster1(context, t)
	ConnectivityCheckNodeToPodCluster2ToCluster1(context, t)
	err := util.DeleteNamespace(context.Client2, namespaceNameCl2)
	assert.Nil(t, err, "error should be nil while deleting namespace %s in cluster %s", namespaceNameCl2, context.ClusterID2)
}

func ConnectivityCheckPodToPodCluster1ToCluster2(con *util.Tester, t *testing.T) {
	localNodes, err := util.GetNodes(con.Client1, con.ClusterID2, labelSelectorNodes)
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	remoteNodes, err := util.GetNodes(con.Client2, con.ClusterID1, labelSelectorNodes)
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	//testing connection from cluster1 to cluster2
	//we expect for the pod to be created on cluster one and also on cluster 2
	//and to communicate with each other
	ns, err := util.CreateNamespace(con.Client1, con.ClusterID1, namespaceNameCl1)
	if err != nil {
		t.Fail()
	}
	reflectedNamespace := ns.Name + "-" + con.ClusterID1
	podRemote := DeployRemotePod(image, podTesterRemoteCl1, ns.Name)
	_, err = con.Client1.CoreV1().Pods(ns.Name).Create(context.TODO(), podRemote, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	podLocal := DeployLocalPod(image, podTesterLocalCl1, ns.Name)
	_, err = con.Client1.CoreV1().Pods(ns.Name).Create(context.TODO(), podLocal, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	if !util.WaitForPodToBeReady("home", con.Client1, waitTime, con.ClusterID1, podLocal.Namespace, podLocal.Name) {
		t.Fail()
	}
	if !util.WaitForPodToBeReady("home", con.Client1, waitTime, con.ClusterID1, podRemote.Namespace, podRemote.Name) {
		t.Fail()
	}
	if !util.WaitForPodToBeReady("foreign", con.Client2, waitTime, con.ClusterID2, reflectedNamespace, podRemote.Name) {
		t.Fail()
	}
	podRemoteUpdateCluster2, err := con.Client2.CoreV1().Pods(reflectedNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", virtualKubelet.ReflectedpodKey, podRemote.Name),
	})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	if len(podRemoteUpdateCluster2.Items) != 1 {
		t.Fatalf("there should be exactly one pod with the label %s", fmt.Sprintf("%s=%s", virtualKubelet.ReflectedpodKey, podRemote.Name))
	}

	podRemoteUpdateCluster1, err := con.Client1.CoreV1().Pods(podRemote.Namespace).Get(context.TODO(), podRemote.Name, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	podLocalUpdate, err := con.Client1.CoreV1().Pods(podLocal.Namespace).Get(context.TODO(), podLocal.Name, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	assert.True(t, isContained(remoteNodes, podRemoteUpdateCluster2.Items[0].Spec.NodeName), "remotepod should be running on one of the local nodes")
	assert.True(t, isContained(localNodes, podLocalUpdate.Spec.NodeName), "localpod should be running on one of the remote pods")
	cmd := command + podRemoteUpdateCluster1.Status.PodIP
	stdout, _, err := util.ExecCmd(con.Config1, con.Client1, podLocalUpdate.Name, podLocalUpdate.Namespace, cmd)
	assert.Equal(t, "200", stdout, "status code should be 200")
	if err != nil {
		t.Fail()
	}
}

func ConnectivityCheckPodToPodCluster2ToCluster1(con *util.Tester, t *testing.T) {
	localNodes, err := util.GetNodes(con.Client2, con.ClusterID2, labelSelectorNodes)
	if err != nil {
		t.Fail()
	}
	remoteNodes, err := util.GetNodes(con.Client1, con.ClusterID1, labelSelectorNodes)
	if err != nil {
		t.Fail()
	}

	//testing connection from cluster2 to cluster1
	//we expect for the pod to be created on cluster one and also on cluster 2
	//and to communicate with each other
	ns, err := util.CreateNamespace(con.Client2, con.ClusterID2, namespaceNameCl2)
	if err != nil {
		t.Fail()
	}
	reflectedNamespace := ns.Name + "-" + con.ClusterID2
	podRemote := DeployRemotePod(image, podTesterRemoteCl2, ns.Name)
	_, err = con.Client2.CoreV1().Pods(ns.Name).Create(context.TODO(), podRemote, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	podLocal := DeployLocalPod(image, podTesterLocalCl2, ns.Name)
	_, err = con.Client2.CoreV1().Pods(ns.Name).Create(context.TODO(), podLocal, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	if !util.WaitForPodToBeReady("home", con.Client2, waitTime, con.ClusterID2, podLocal.Namespace, podLocal.Name) {
		t.Fail()
	}
	if !util.WaitForPodToBeReady("home", con.Client2, waitTime, con.ClusterID2, podRemote.Namespace, podRemote.Name) {
		t.Fail()
	}
	if !util.WaitForPodToBeReady("foreign", con.Client1, waitTime, con.ClusterID1, reflectedNamespace, podRemote.Name) {
		t.Fail()
	}
	podRemoteUpdateCluster1, err := con.Client1.CoreV1().Pods(reflectedNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", virtualKubelet.ReflectedpodKey, podRemote.Name),
	})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	if len(podRemoteUpdateCluster1.Items) != 1 {
		klog.Errorf("there should be exactly one pod with the label %s", fmt.Sprintf("%s=%s", virtualKubelet.ReflectedpodKey, podRemote.Name))
		t.Fail()
	}

	podRemoteUpdateCluster2, err := con.Client2.CoreV1().Pods(podRemote.Namespace).Get(context.TODO(), podRemote.Name, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	podLocalUpdate, err := con.Client2.CoreV1().Pods(podLocal.Namespace).Get(context.TODO(), podLocal.Name, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}

	assert.True(t, isContained(remoteNodes, podRemoteUpdateCluster1.Items[0].Spec.NodeName), "remotepod should be running on one of the local nodes")
	assert.True(t, isContained(localNodes, podLocalUpdate.Spec.NodeName), "localpod should be running on one of the remote pods")
	cmd := command + podRemoteUpdateCluster2.Status.PodIP
	stdout, _, err := util.ExecCmd(con.Config2, con.Client2, podLocalUpdate.Name, podLocalUpdate.Namespace, cmd)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, "200", stdout, "status code should be 200")
	if err != nil {
		t.Fail()
	}
}

func ConnectivityCheckNodeToPodCluster1ToCluster2(con *util.Tester, t *testing.T) {
	nodePort, err := util.CreateNodePort(con.Client1, con.ClusterID1, podTesterRemoteCl1, "nodeport-cl1", namespaceNameCl1)
	if err != nil {
		t.Fail()
	}
	localNodes, err := util.GetNodes(con.Client1, con.ClusterID1, labelSelectorNodes)
	if err != nil {
		t.Fail()
	}
	time.Sleep(10 * time.Second)
	for _, node := range localNodes.Items {
		cmd := command + node.Status.Addresses[0].Address + ":" + strconv.Itoa(int(nodePort.Spec.Ports[0].NodePort))
		c := exec.Command("sh", "-c", cmd)
		output := &bytes.Buffer{}
		errput := &bytes.Buffer{}
		c.Stdout = output
		c.Stderr = errput
		klog.Infof("running command %s", cmd)
		err := c.Run()
		if err != nil {
			klog.Error(err)
			klog.Infof(errput.String())
			t.Fail()
		}
		assert.Nil(t, err, "error should be nil")
		assert.Equal(t, "200", output.String(), "status code should be 200")
	}
}

func ConnectivityCheckNodeToPodCluster2ToCluster1(con *util.Tester, t *testing.T) {
	nodePort, err := util.CreateNodePort(con.Client2, con.ClusterID2, podTesterRemoteCl2, "nodeport-cl2", namespaceNameCl2)
	if err != nil {
		t.Fail()
	}
	localNodes, err := util.GetNodes(con.Client2, con.ClusterID2, labelSelectorNodes)
	if err != nil {
		t.Fail()
	}
	time.Sleep(10 * time.Second)
	for _, node := range localNodes.Items {
		cmd := command + node.Status.Addresses[0].Address + ":" + strconv.Itoa(int(nodePort.Spec.Ports[0].NodePort))
		c := exec.Command("sh", "-c", cmd)
		output := &bytes.Buffer{}
		errput := &bytes.Buffer{}
		c.Stdout = output
		c.Stderr = errput
		klog.Infof("running command %s", cmd)
		err := c.Run()
		if err != nil {
			klog.Error(err)
			klog.Infof(errput.String())
			t.Fail()
		}
		assert.Nil(t, err, "error should be nil")
		assert.Equal(t, "200", output.String(), "status code should be 200")
	}
}

func DeployRemotePod(image, podName, namespace string) *v1.Pod {
	pod1 := v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    map[string]string{"app": podName},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "tester",
					Image:           image,
					Resources:       v1.ResourceRequirements{},
					ImagePullPolicy: "IfNotPresent",
					Ports: []v1.ContainerPort{{
						ContainerPort: 80,
					}},
				},
			},
			Affinity: &v1.Affinity{
				NodeAffinity: &v1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{NodeSelectorTerms: []v1.NodeSelectorTerm{{
						MatchExpressions: []v1.NodeSelectorRequirement{{
							Key:      liqocontrollerutils.TypeLabel,
							Operator: "In",
							Values:   []string{liqocontrollerutils.TypeNode},
						}},
						MatchFields: nil,
					}}},
				},
			},
		},
		Status: v1.PodStatus{},
	}
	return &pod1
}

func DeployLocalPod(image, podName, namespace string) *v1.Pod {
	pod2 := v1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "tester",
					Image:           image,
					ImagePullPolicy: "IfNotPresent",
					Ports: []v1.ContainerPort{{
						ContainerPort: 80,
					},
					},
				}},

			Affinity: &v1.Affinity{
				NodeAffinity: &v1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{NodeSelectorTerms: []v1.NodeSelectorTerm{{
						MatchExpressions: []v1.NodeSelectorRequirement{{
							Key:      liqocontrollerutils.TypeLabel,
							Operator: "NotIn",
							Values:   []string{liqocontrollerutils.TypeNode},
						}},
						MatchFields: nil,
					}}},
				},
			},
		},
	}
	return &pod2
}

func isContained(nodes *v1.NodeList, nodeName string) bool {
	for _, node := range nodes.Items {
		if nodeName == node.Name {
			return true
		}
	}
	return false
}
