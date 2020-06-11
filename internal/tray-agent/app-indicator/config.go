package app_indicator

import (
	"os"
	"path/filepath"
)
// data structure containing indicator configuration
type config struct {
	notifyLevel    NotifyLevel
	notifyIconPath string
}
// newConfig assign a startup configuration to the Indicator
func newConfig() *config {
	//home,set := os.LookupEnv("HOME")
	liqoPath := filepath.Join(os.Getenv("HOME"), "liqo")
	if err := os.Setenv("LIQO_PATH", liqoPath); err != nil {
		os.Exit(1)
	}
	conf := config{notifyLevel: NotifyLevelMax, notifyIconPath: filepath.Join(liqoPath, "icons")}
	return &conf
}

