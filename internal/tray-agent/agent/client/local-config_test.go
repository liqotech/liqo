package client

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalConfiguration(t *testing.T) {
	//set env variables
	env, present := os.LookupEnv(EnvLiqoPath)
	liqoPath, err := filepath.Abs("test_config/liqo")
	if err != nil {
		t.Fatal(err)
	}
	assert.NoError(t, os.Setenv(EnvLiqoPath, liqoPath), "PRE-TEST: envLiqoPath not set")
	assert.NoError(t, os.MkdirAll(liqoPath, 0777), "PRE-TEST: path for Liqo directory not created")

	NewLocalConfig()
	conf, valid := GetLocalConfig()
	assert.False(t, valid, "new configuration should not be valid")
	assert.NotNil(t, conf.Content, "content for new configuration should not be nil")
	setString := "/test/path"
	//update local configuration
	conf.SetKubeconfig(setString)
	assert.Equal(t, setString, conf.Content.Kubeconfig, "configuration content not updated")
	//write to file
	assert.NoError(t, SaveLocalConfig(), "error on file writing")
	//reset local configuration
	NewLocalConfig()
	//read from file
	LoadLocalConfig()
	conf, valid = GetLocalConfig()
	assert.True(t, valid, "correct loaded configuration should be valid")
	//read from local configuration
	getString := conf.GetKubeconfig()
	assert.Equal(t, setString, getString, "loaded configuration differs from saved one")
	//POST TEST: delete file
	_ = os.RemoveAll(EnvLiqoPath)
	//POST TEST: reset env var
	if present {
		_ = os.Setenv(EnvLiqoPath, env)
	}
}
