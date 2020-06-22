package app_indicator

import (
	"os"
	"path/filepath"
)

// data structure containing indicator configuration
type config struct {
	notifyLevel            NotifyLevel
	notifyIconPath         string
	notifyTranslate        map[NotifyLevel]string
	notifyTranslateReverse map[string]NotifyLevel
}

func (c *config) NotifyTranslateReverse() map[string]NotifyLevel {
	return c.notifyTranslateReverse
}

func (c *config) NotifyTranslate() map[NotifyLevel]string {
	return c.notifyTranslate
}

func (c *config) NotifyLevel() NotifyLevel {
	return c.notifyLevel
}

// newConfig assign a startup configuration to the Indicator
func newConfig() *config {
	//home,set := os.LookupEnv("HOME")
	liqoPath := filepath.Join(os.Getenv("HOME"), "liqo")
	if err := os.Setenv("LIQO_PATH", liqoPath); err != nil {
		os.Exit(1)
	}
	conf := &config{notifyLevel: NotifyLevelMax, notifyIconPath: filepath.Join(liqoPath, "icons")}
	conf.notifyTranslate = make(map[NotifyLevel]string)
	conf.notifyTranslateReverse = make(map[string]NotifyLevel)
	conf.notifyTranslate[NotifyLevelOff] = "Notifications OFF"
	conf.notifyTranslate[NotifyLevelMin] = "Notify with icon"
	conf.notifyTranslate[NotifyLevelMax] = "Notify with icon and banner"
	for k, v := range conf.notifyTranslate {
		conf.notifyTranslateReverse[v] = k
	}
	return conf
}
