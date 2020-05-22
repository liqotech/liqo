package discovery

import (
	"errors"
	"github.com/netgroup-polito/dronev2/internal/discovery/clients"
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

	AutoFederation bool `json:"autoFederation"`
}

func GetDiscoveryConfig() *Config {
	dc := &Config{}

	client, err := clients.NewK8sClient()
	if err != nil {
		Log.Error(err, err.Error())
		os.Exit(1)
	}

	configMap, err := client.CoreV1().ConfigMaps(Namespace).Get("discovery-config", metav1.GetOptions{})
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

	dc.Name = config["name"]
	dc.Service = config["service"]
	dc.Domain = config["domain"]
	dc.Port, err = strconv.Atoi(config["port"])
	if err != nil {
		Log.Error(err, err.Error())
		os.Exit(1)
	}

	dc.EnableDiscovery = config["enableDiscovery"] == "true"
	dc.EnableAdvertisement = config["enableAdvertisement"] == "true"

	dc.AutoFederation = config["autoFederation"] == "true"

	if dc.EnableAdvertisement {
		dc.TxtData = GetTxtData()
	}

	dc.WaitTime, err = strconv.Atoi(config["waitTime"]) // wait response time
	if err != nil {
		Log.Error(err, err.Error())
		os.Exit(1)
	}
	dc.UpdateTime, err = strconv.Atoi(config["updateTime"]) // time between update queries
	if err != nil {
		Log.Error(err, err.Error())
		os.Exit(1)
	}

	return dc
}

func checkConfig(config map[string]string) error {
	reqFields := []string{"name", "service", "domain", "port", "enableDiscovery", "enableAdvertisement", "autoFederation", "waitTime", "updateTime"}
	for _, f := range reqFields {
		if config[f] == "" {
			return errors.New("Missing required field " + f)
		}
	}
	return nil
}
