package kubeconfig

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"github.com/netgroup-polito/dronev2/internal/discovery/clients"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetLocalClusterID() (string, error) {
	// TODO: return ClusterID when we will have a way to create and get it
	// this is only a placeholder to avoid collisions
	client, err := clients.NewK8sClient()
	if err != nil {
		return "", err
	}
	nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master=true",
	})
	if err != nil {
		return "", err
	}
	ip := ""
	for _, addr := range nodes.Items[0].Status.Addresses {
		if addr.Type == "InternalIP" {
			ip = addr.Address
		}
	}
	if ip == "" {
		return "", errors.New("master IP not found")
	}
	hasher := sha1.New()
	hasher.Write([]byte(ip))
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
