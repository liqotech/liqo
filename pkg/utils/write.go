package utils

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"os"
)

func WriteFile(filepath string, content []byte) error {
	f, err := os.Create(filepath)
	if err != nil {
		klog.Errorf("Unable to create file: %s", err)
		return err
	}

	defer f.Close()

	_, err = f.Write(content)
	if err != nil {
		klog.Errorf("Unable to write certificate file: %s", err)
		return err
	}
	return nil
}

func UserConfig(configPath string) (*rest.Config, error) {
	var config *rest.Config
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		// Get the kubeconfig from the filepath.
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return nil, err
		}
	} else {
		// Set to in-cluster config.
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}
	return config, nil
}
