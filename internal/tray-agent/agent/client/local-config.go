package client

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

//ConfigFilename is the basename of the Agent configuration file.
const ConfigFilename = "agent_conf.yaml"

//fileConfig contains Liqo Agent configuration parameters acquired from the cluster.
var fileConfig *LocalConfiguration

//LocalConfig maps the information of a Liqo Agent configuration file, containing persistent settings data.
type LocalConfig struct {
	//Kubeconfig contains the path of the kubeconfig file.
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
}

//LocalConfiguration stores the LocalConfig configuration acquired from a local config file and a validity flag.
type LocalConfiguration struct {
	//Content maps the content of the config file.
	Content *LocalConfig
	//Valid specifies whether LocalConfiguration contains a valid Content.
	Valid bool
	sync.RWMutex
}

func newLocalConfiguration() *LocalConfiguration {
	return &LocalConfiguration{
		Content: &LocalConfig{},
	}
}

//LoadLocalConfig tries to load configuration data from a config file ConfigFilename on the local filesystem
//(if present and valid). The config file structure is mapped on the LocalConfig type.
func LoadLocalConfig() {
	fileConfig := newLocalConfiguration()
	liqoDir, present := os.LookupEnv("LIQO_PATH")
	if !present {
		return
	}
	filePath := filepath.Join(liqoDir, ConfigFilename)
	yamlFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		return
	}
	err = yaml.Unmarshal(yamlFile, fileConfig.Content)
	if err != nil {
		return
	}
	fileConfig.Valid = true
}

//SaveLocalConfig tries to save the configuration data contained in the AgentController inner LocalConfig to a
//config file one the local file system named after ConfigFilename.
func SaveLocalConfig() {
	liqoDir, present := os.LookupEnv("LIQO_PATH")
	if !present {
		return
	}
	if _, err := os.Stat(liqoDir); err != nil || fileConfig == nil {
		return
	}
	data, err := yaml.Marshal(fileConfig.Content)
	if err != nil {
		return
	}
	_ = ioutil.WriteFile(filepath.Join(liqoDir, ConfigFilename), data, 0644)
}

//GetLocalConfig returns configuration data acquired from a config file on the local file system.
func GetLocalConfig() (config *LocalConfiguration, valid bool) {
	if fileConfig == nil {
		return nil, false
	}
	return fileConfig, fileConfig.Valid
}
