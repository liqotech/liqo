package discovery

import (
	v1 "github.com/netgroup-polito/dronev2/pkg/discovery/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"path/filepath"
)

// Read ConfigMap and start register and resolver goroutines
func StartDiscovery() {
	dc := GetDiscoveryConfig()

	var txt []string
	if dc.EnableAdvertisement {
		txtString, err := dc.TxtData.Encode()
		if err != nil {
			log.Println(err.Error())
			os.Exit(1)
		}
		txt = []string{txtString}
	}

	if dc.EnableAdvertisement {
		log.Println("Starting service advertisement")
		go Register(dc.Name, dc.Service, dc.Domain, dc.Port, txt)
	}

	if dc.EnableDiscovery {
		log.Println("Starting service discovery")
		go StartResolver(dc.Service, dc.Domain, dc.WaitTime, dc.UpdateTime)
	}
}

func NewConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	}
	return config, err
}

func NewK8sClient() (*kubernetes.Clientset, error) {
	config, err := NewConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func NewDiscoveryClient() (*v1.V1Client, error) {
	config, err := NewConfig()
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
	return v1.NewForConfig(config)
}
