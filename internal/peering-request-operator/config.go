package peering_request_operator

import (
	"errors"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"
)

type Config struct {
	AllowAll bool `json:"allowAll"`
}

func GetConfig(client *kubernetes.Clientset, Log logr.Logger, namespace string) *Config {
	conf := &Config{}

	configMap, err := client.CoreV1().ConfigMaps(namespace).Get("peering-request-operatorn-cm", metav1.GetOptions{})
	if err != nil {
		Log.Error(err, err.Error())
		os.Exit(1)
	}

	config := configMap.Data

	err = checkConfig(config)
	if err != nil {
		Log.Error(err, err.Error())
		os.Exit(1)
	}

	conf.AllowAll = config["allowAll"] == "true"

	return conf
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
