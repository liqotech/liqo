package peering_request_operator

import (
	"errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

type Config struct {
	AllowAll bool `json:"allowAll"`
}

func GetConfig(client *kubernetes.Clientset, namespace string) (*Config, error) {
	conf := &Config{}

	configMap, err := client.CoreV1().ConfigMaps(namespace).Get("peering-request-operator-cm", metav1.GetOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		return nil, err
	}

	config := configMap.Data

	err = checkConfig(config)
	if err != nil {
		klog.Error(err, err.Error())
		return nil, err
	}

	conf.AllowAll = config["allowAll"] == "true"

	return conf, nil
}

func checkConfig(config map[string]string) error {
	reqFields := []string{"allowAll"}
	for _, f := range reqFields {
		if config[f] == "" {
			return errors.New("Missing required field " + f)
		}
	}
	return nil
}
