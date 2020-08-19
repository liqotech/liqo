package util

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sync"
	"k8s.io/klog"
	"os"
)

type Tester struct {
	Client1   *kubernetes.Clientset
	Client2   *kubernetes.Clientset
	Namespace string
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
		Client1:   clientset1,
		Client2:   clientset2,
		Namespace: namespace,
	}
}