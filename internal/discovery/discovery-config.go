package discovery

import (
	"errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"strconv"
)

type Config struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	Domain  string `json:"domain"`
	Port    int    `json:"port"`

	TxtData TxtData `json:"txtData"`

	WaitTime   int `json:"waitTime"`
	UpdateTime int `json:"updateTime"`

	EnableDiscovery     bool `json:"enableDiscovery"`
	EnableAdvertisement bool `json:"enableAdvertisement"`

	AutoJoin bool `json:"autojoin"`
}

func (discovery *DiscoveryCtrl) GetDiscoveryConfig() {
	configMap, err := discovery.client.CoreV1().ConfigMaps(discovery.Namespace).Get("discovery-config", metav1.GetOptions{})
	if err != nil {
		discovery.Log.Error(err, err.Error())
		os.Exit(1)
	}

	config := configMap.Data

	err = checkConfig(config)
	if err != nil {
		discovery.Log.Error(err, err.Error())
		os.Exit(1)
	}

	discovery.config.Name = config["name"]
	discovery.config.Service = config["service"]
	discovery.config.Domain = config["domain"]
	discovery.config.Port, err = strconv.Atoi(config["port"])
	if err != nil {
		discovery.Log.Error(err, err.Error())
		os.Exit(1)
	}

	discovery.config.EnableDiscovery = config["enableDiscovery"] == "true"
	discovery.config.EnableAdvertisement = config["enableAdvertisement"] == "true"

	discovery.config.AutoJoin = config["autoJoin"] == "true"

	if discovery.config.EnableAdvertisement {
		discovery.config.TxtData = discovery.GetTxtData()
	}

	discovery.config.WaitTime, err = strconv.Atoi(config["waitTime"]) // wait response time
	if err != nil {
		discovery.Log.Error(err, err.Error())
		os.Exit(1)
	}
	discovery.config.UpdateTime, err = strconv.Atoi(config["updateTime"]) // time between update queries
	if err != nil {
		discovery.Log.Error(err, err.Error())
		os.Exit(1)
	}
}

func checkConfig(config map[string]string) error {
	reqFields := []string{"name", "service", "domain", "port", "enableDiscovery", "enableAdvertisement", "autoJoin", "waitTime", "updateTime"}
	for _, f := range reqFields {
		if config[f] == "" {
			return errors.New("Missing required field " + f)
		}
	}
	return nil
}
