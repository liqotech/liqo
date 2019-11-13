package configuration

import (
"gopkg.in/yaml.v2"
"io/ioutil"
"path/filepath"
)

type ConfigType struct {
	RabbitConf struct {
		BrokerAddress string `yaml:"broker_address"`
		BrokerPort string `yaml:"broker_port"`
		VirtualHost string `yaml:"v_host"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		QueueResources string `yaml:"queue_resources"`
		QueueAdvertisement string `yaml:"queue_adv_route"`
		QueueUserRequest string `yaml:"queue_user_req"`
		QueueResult string `yaml:"queue_result"`
		QueueAdvertisementCtrl string `yaml:"queue_adv_ctrl"`
		QueueAdvertisementDrone string `yaml:"queue_adv_drone"`
		QueueAcknowledgeDeploy string `yaml:"queue_ack_deploy"`
	} `yaml:"rabbit"`
	Federation struct {
		ExchangeName string `yaml:"exchange_name"`
		SetName string `yaml:"set_name"`
		PolicyName string `yaml:"policy_name"`
		Pattern string `yaml:"pattern"`
	} `yaml:"federation"`
	Kubernetes struct {
		Namespace string `yaml:"namespace"`
		ClusterName string `yaml:"cluster_name"`
	} `yaml:"kubernetes"`
	Folder struct{
		YamlFolder string `yaml:"yaml_folder"`
	}`yaml:"folder"`
}

func Config() *ConfigType{
	c:= new(ConfigType)

	//filename, _ := filepath.Abs("pkg/controller/common/configuration/conf.yml")//("/etc/config/conf.yaml")
	filename, _ := filepath.Abs("/etc/config/conf.yaml")
	yamlFile, err := ioutil.ReadFile(filename)
	err = yaml.Unmarshal(yamlFile, &c)
	if err != nil {
		panic(err)
	}

	return c
}