package discovery

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
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

	client, err := NewK8sClient()
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	configMap, err := client.CoreV1().ConfigMaps("default").Get("discovery-config", metav1.GetOptions{})
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	config := configMap.Data

	// TODO: check if config has required fields

	dc.Name = config["name"]
	dc.Service = config["service"]
	dc.Domain = config["domain"]
	dc.Port, err = strconv.Atoi(config["port"])
	if err != nil {
		log.Println(err, "Unable to get configMap")
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
		log.Println(err.Error())
		os.Exit(1)
	}
	dc.UpdateTime, err = strconv.Atoi(config["updateTime"]) // time between update queries
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	return dc
}
