package client

import (
	"errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

//ConfigFileName is the basename of the Agent configuration file.
const ConfigFileName = "agent_conf.yaml"

//fileConfig contains Liqo Agent configuration parameters acquired from the cluster.
var fileConfig = &LocalConfiguration{}

//LocalConfig maps the information of a Liqo Agent configuration file, containing persistent settings data.
type LocalConfig struct {
	//Kubeconfig contains the path of the kubeconfig file.
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
}

//LocalConfiguration stores the LocalConfig configuration acquired from a local config file and a validity flag.
type LocalConfiguration struct {
	//Content maps the content of the config file.
	Content *LocalConfig
	//Valid specifies whether LocalConfiguration contains a valid Content to read.
	Valid bool
	sync.RWMutex
}

//NewLocalConfig clears Agent internal copy of the ConfigFileName config file.
func NewLocalConfig() *LocalConfiguration {
	fileConfig.Lock()
	defer fileConfig.Unlock()
	fileConfig.Content = &LocalConfig{}
	return fileConfig
}

//LoadLocalConfig loads configuration data from a config file ConfigFileName on the local filesystem
//(if present and valid). The config file structure is mapped on the LocalConfig type.
func LoadLocalConfig() {
	lc := NewLocalConfig()
	liqoDir, present := os.LookupEnv(EnvLiqoPath)
	if !present {
		return
	}
	filePath := filepath.Join(liqoDir, ConfigFileName)
	yamlFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		return
	}
	lc.Lock()
	defer lc.Unlock()
	err = yaml.Unmarshal(yamlFile, lc.Content)
	if err != nil {
		return
	}
	lc.Valid = true
}

//SaveLocalConfig saves the configuration data in the internal LocalConfiguration to a
//config file on the local file system named after ConfigFileName.
func SaveLocalConfig() error {
	liqoDir, present := os.LookupEnv(EnvLiqoPath)
	if !present {
		return errors.New("envLiqoPath not set")
	}
	fileConfig.RLock()
	defer fileConfig.RUnlock()
	if _, err := os.Stat(liqoDir); err != nil {
		return err
	}
	if fileConfig.Content == nil {
		return errors.New("trying to save nil configuration")
	}
	data, err := yaml.Marshal(fileConfig.Content)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(liqoDir, ConfigFileName), data, 0644)
}

//GetLocalConfig returns configuration data acquired from a config file on the local file system.
func GetLocalConfig() (config *LocalConfiguration, valid bool) {
	fileConfig.RLock()
	defer fileConfig.RUnlock()
	if fileConfig.Content == nil {
		return fileConfig, false
	}
	return fileConfig, fileConfig.Valid
}

//LocalConfig getters and setters.
/* These methods are required to ensure a safe access between goroutines. */

//GetKubeconfig returns the 'kubeconfig' field for the local configuration.
func (lc *LocalConfiguration) GetKubeconfig() string {
	lc.RLock()
	defer lc.RUnlock()
	if lc.Content == nil {
		return ""
	}
	return lc.Content.Kubeconfig
}

//SetKubeconfig sets the 'kubeconfig' field for the local configuration. Use SaveLocalConfig to write the updated
//configuration to the ConfigFileName file.
func (lc *LocalConfiguration) SetKubeconfig(path string) {
	lc.Lock()
	defer lc.Unlock()
	if lc.Content == nil {
		lc.Content = &LocalConfig{Kubeconfig: path}
		return
	}
	lc.Content.Kubeconfig = path
}
