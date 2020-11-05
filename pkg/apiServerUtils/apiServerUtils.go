package apiServerUtils

import (
	"context"
	"errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"os"
)

func GetAddress(clientset kubernetes.Interface) (string, error) {
	address, ok := os.LookupEnv("APISERVER")
	if !ok || address == "" {
		nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		if err != nil {
			return "", err
		}
		if len(nodes.Items) == 0 || len(nodes.Items[0].Status.Addresses) == 0 {
			err = errors.New("no APISERVER env variable found and no master node found, one of the two values must be present")
			klog.Error(err)
			return "", err
		}
		address = nodes.Items[0].Status.Addresses[0].Address
	}
	return address, nil
}

func GetPort() string {
	port, ok := os.LookupEnv("APISERVER_PORT")
	if !ok {
		port = "6443"
	}
	return port
}
