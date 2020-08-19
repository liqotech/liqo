package util

import (
	"bytes"
	"context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/api/v1/pod"
	api "k8s.io/kubernetes/pkg/apis/core"
	"os"
	"sync"
	"time"
)

type Tester struct {
	Config1    *rest.Config
	Config2    *rest.Config
	Client1    *kubernetes.Clientset
	Client2    *kubernetes.Clientset
	ClusterID1 string
	ClusterID2 string
	Namespace  string
}

var l = &sync.Mutex{}

var (
	t1               *Tester
	clusterIDConfMap = "cluster-id"
	namespaceLabels  = map[string]string{"liqo.io/enabled": "true"}
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
	clusterID1, err := getClusterID(clientset1, namespace)
	if err != nil {
		klog.Errorf("an error occurred while getting cluster-id configmap %s", err)
		os.Exit(1)
	}
	clusterID2, err := getClusterID(clientset2, namespace)
	if err != nil {
		klog.Errorf("an error occurred while getting cluster-id configmap %s", err)
		os.Exit(1)
	}
	return &Tester{
		Config1:    config1,
		Config2:    config2,
		Client1:    clientset1,
		Client2:    clientset2,
		Namespace:  namespace,
		ClusterID1: clusterID1,
		ClusterID2: clusterID2,
	}
}

func WaitForPodToBeReady(client *kubernetes.Clientset, waitSeconds time.Duration, clusterID, namespace, podName string) bool {
	stop := make(chan bool, 1)
	ready := make(chan bool, 1)
	klog.Infof("%s -> waiting for pod %s on namespace %s to become ready", clusterID, podName, namespace)
	go PodWatcher(client, clusterID, namespace, podName, ready, stop)
	select {
	case isReady := <-ready:
		return isReady
	case <-time.After(waitSeconds):
		klog.Infof("%s -> pod %s on namespace %s required more than %s to be ready", clusterID, podName, namespace, waitSeconds)
		return false
	}
}

func PodWatcher(client *kubernetes.Clientset, clusterID, namespace, podName string, podReady, stopCh chan bool) {
	watcher, err := client.CoreV1().Pods(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(api.ObjectNameField, podName).String(),
	})
	if err != nil {
		klog.Errorf("%s -> an error occurred while launching the watcher for pod %s: %s", clusterID, podName, err)
		return
	}
	event := watcher.ResultChan()
	for {
		select {
		case <-stopCh:
			klog.Infof("%s -> the watcher for pod %s timed out", clusterID, podName)
			podReady <- false
		case e := <-event:
			obj, ok := e.Object.(*v1.Pod)
			if !ok {
				klog.Infof("object is not a pod")
				continue
			}
			switch e.Type {
			case watch.Added:
				if pod.IsPodReady(obj) {
					podReady <- true
					return
				}
			case watch.Modified:
				if pod.IsPodReady(obj) {
					podReady <- true
					return
				}
			case watch.Deleted:
				podReady <- false
				return
			}
		}
	}
}

func getClusterID(client *kubernetes.Clientset, namespace string) (string, error) {
	cmClient := client.CoreV1().ConfigMaps(namespace)
	cm, err := cmClient.Get(context.TODO(), clusterIDConfMap, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	clusterID := cm.Data[clusterIDConfMap]
	klog.Infof("got clusterID %s", clusterID)
	return clusterID, nil
}

func ExecCmd(config *rest.Config, client *kubernetes.Clientset, podName, namespace, command string) (string, string, error) {

	cmd := []string{
		"sh",
		"-c",
		command,
	}
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")
	req.VersionedParams(&v1.PodExecOptions{
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
		Command: cmd,
	}, scheme.ParameterCodec)

	var stdout, stderr bytes.Buffer
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", err
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return "", "", err
	}
	return stdout.String(), stderr.String(), err

}

func CreateNamespace(client *kubernetes.Clientset, clusterID string, name string) (*v1.Namespace, error) {
	ns := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: namespaceLabels,
		},
		Spec:   v1.NamespaceSpec{},
		Status: v1.NamespaceStatus{},
	}
	ns, err := client.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("%s -> an error occurred while creating namespace %s : %s", clusterID, name, err)
		return nil, err
	}
	return ns, nil
}

func DeleteNamespace(client *kubernetes.Clientset, name string) error {
	err := client.CoreV1().Namespaces().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func GetNodes(client *kubernetes.Clientset, clusterID string, labelSelector string) (*v1.NodeList, error) {
	remoteNodes, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		klog.Errorf("%s -> an error occurred while listing nodes: %s", clusterID, err)
		return nil, err
	}
	return remoteNodes, nil
}

func CreateNodePort(client *kubernetes.Clientset, clusterID, appName, name, namespace string) (*v1.Service, error) {
	nodePort := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{
				Name:        "http",
				Protocol:    "TCP",
				AppProtocol: nil,
				Port:        80,
				TargetPort: intstr.IntOrString{
					IntVal: 80,
				},
			}},
			Selector: map[string]string{"app": appName},
			Type:     v1.ServiceTypeNodePort,
		},
		Status: v1.ServiceStatus{},
	}
	nodePort, err := client.CoreV1().Services(namespace).Create(context.TODO(), nodePort, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("%s -> an error occurred while creating nodePort %s in namespace %s: %s", clusterID, name, namespace, err)
		return nil, err
	}
	return nodePort, nil
}
