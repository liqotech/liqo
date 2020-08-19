package net_test

import (
	"context"
	"github.com/liqoTech/liqo/test/e2e/util"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
	"os"
	"testing"
	v1 "k8s.io/api/core/v1"
	"time"
	"k8s.io/klog"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/kubernetes/scheme"
	"io"
)

func TestPodConnectivity1to2(t *testing.T) {
	context := util.GetTester()
	ConnectivityCheck(context.Client1, context.Client2, t, "cluster1")
}

func ConnectivityCheck(c1 *kubernetes.Clientset, c2 *kubernetes.Clientset, t *testing.T, namespace string){
	ns := v1.Namespace{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "test-connectivity",
		},
		Spec:       v1.NamespaceSpec{},
		Status:     v1.NamespaceStatus{},
	}
	remoteNodes, err := c1.CoreV1().Nodes().List(context.TODO(),metav1.ListOptions{
		LabelSelector:       "virtual-node=true",
	})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	localNodes, err := c1.CoreV1().Nodes().List(context.TODO(),metav1.ListOptions{
		LabelSelector:       "virtual-node!=true",
	})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	_, err = c1.CoreV1().Namespaces().Create(context.TODO(),&ns,metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	p1 := DeployLocalPod()
	_, err = c1.CoreV1().Pods("test-connectivity").Create(context.TODO(),p1,metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	p2 := DeployRemotePod()
	_, err = c1.CoreV1().Pods("test-connectivity").Create(context.TODO(),p2,metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	time.Sleep(5*time.Second)
	p1Update, err := c1.CoreV1().Pods("test-connectivity").Get(context.TODO(),"tester-local", metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
	p2Update, err := c1.CoreV1().Pods("test-connectivity").Get(context.TODO(),"tester-remote",metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		t.Fail()
	}
    assert.Equal(t,p2Update.Spec.NodeName,remoteNodes.Items[0].Name)
	assert.Equal(t,p1Update.Spec.NodeName,localNodes.Items[0].Name)
	cmd := "curl --fail http://" + p2.Status.PodIP + "/healthz"
	err = ExecCmdExample(c1,&restclient.Config{},p1.Name,cmd,os.Stdin,os.Stdout,os.Stderr)
	assert.Equal(t,err,"")
}

// ExecCmd exec command on specific pod and wait the command's output.
func ExecCmdExample(client kubernetes.Interface, config *restclient.Config, podName string,
	command string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	cmd := []string{
		"sh",
		"-c",
		command,
	}
	req := client.CoreV1().RESTClient().Post().Resource("pods").Name(podName).
		Namespace("default").SubResource("exec")
	option := &v1.PodExecOptions{
		Command: cmd,
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}
	if stdin == nil {
		option.Stdin = false
	}
	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		return err
	}

	return nil
}

func DeployLocalPod() *v1.Pod {
	pod1 := v1.Pod{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "tester-local",
			Namespace:                  "",
		},
		Spec:       v1.PodSpec{
			Containers:                   []v1.Container{
				{
					Name:                     "Tester",
					Image:                    "nginx",
					Resources:                v1.ResourceRequirements{},
					ImagePullPolicy:          "IfNotPresent",
				},
			},
			NodeSelector: map[string]string{
				"virtual-node": "false",
			},
		},
		Status:     v1.PodStatus{},
	}
	return &pod1
}

func DeployRemotePod() *v1.Pod {
	pod2 := v1.Pod{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "tester-local",
			Namespace:                  "",
		},
		Spec:       v1.PodSpec{
			Containers:                   []v1.Container{
				{
					Name:                     "Tester",
					Image:                    "nginx",
					ImagePullPolicy:          "IfNotPresent",
				},
			},
			NodeSelector: map[string]string{
				"virtual-node": "true",
			},
		},
	}
	return &pod2
}