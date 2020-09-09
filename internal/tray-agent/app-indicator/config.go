package app_indicator

import (
	"os"
	"path/filepath"
)

// data structure containing Indicator configuration
type config struct {
	// current setting for the notification system
	notifyLevel NotifyLevel
	// filesystem path of the directory containing the icons used in the desktop banners
	notifyIconPath string
	// map that translates a NotifyLevel into its correspondent user-friendly literal description
	notifyTranslateMap map[NotifyLevel]string
	// map that performs the reverse translation of the notifyTranslateMap map
	notifyTranslateReverseMap map[string]NotifyLevel
}

// newConfig assigns a startup configuration to the Indicator
func newConfig() *config {
	liqoPath := filepath.Join(os.Getenv("HOME"), ".liqo")
	if err := os.Setenv("LIQO_PATH", liqoPath); err != nil {
		os.Exit(1)
	}
	conf := &config{notifyLevel: NotifyLevelMax, notifyIconPath: filepath.Join(liqoPath, "icons")}
	conf.notifyTranslateMap = make(map[NotifyLevel]string)
	conf.notifyTranslateReverseMap = make(map[string]NotifyLevel)
	conf.notifyTranslateMap[NotifyLevelOff] = NotifyLevelOffDescription
	conf.notifyTranslateMap[NotifyLevelMin] = NotifyLevelMinDescription
	conf.notifyTranslateMap[NotifyLevelMax] = NotifyLevelMaxDescription
	for k, v := range conf.notifyTranslateMap {
		conf.notifyTranslateReverseMap[v] = k
	}
	return conf
}

//Config returns the Indicator config.
func (i *Indicator) Config() *config {
	return i.config
}

//NotifyTranslate translates a NotifyLevel into its correspondent user-friendly textual description.
//If such level does not exist, it returns NotifyLevelUnknown
func (c *config) NotifyTranslate(level NotifyLevel) string {
	description, exist := c.notifyTranslateMap[level]
	if !exist {
		description = notifyLevelUnknownDescription
	}
	return description
}

//NotifyTranslateReverse translates the textual description of a Notification mode into its correspondent
//NotifyLevel
func (c *config) NotifyTranslateReverse(levelDescription string) NotifyLevel {
	l, exist := c.notifyTranslateReverseMap[levelDescription]
	if !exist {
		l = notifyLevelUnknown
	}
	return l
}

//NotifyDescriptions returns the user-friendly textual description of all the NotifyLevel available for the
//Indicator. Descriptions are sorted in ascending order by NotifyLevel.
func (c *config) NotifyDescriptions() []string {
	// retrieving the descriptions from the notifyTranslateMap provides
	// an automatic sorting based on the notification level
	descriptions := make([]string, 0)
	for _, v := range c.notifyTranslateMap {
		descriptions = append(descriptions, v)
	}
	return descriptions
}

//NotifyLevel returns the current notification mode for the Indicator. Its value affects the Indicator.Notify()
//method behavior.
func (c *config) NotifyLevel() NotifyLevel {
	return c.notifyLevel
}
