package app_indicator

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

// test config translation capabilities between NotifyLevel and textual representation of
// Indicator notification modes
func TestNotifyTranslations(t *testing.T) {
	conf := &config{
		notifyTranslateMap:        make(map[NotifyLevel]string),
		notifyTranslateReverseMap: make(map[string]NotifyLevel),
	}
	conf.notifyTranslateMap[NotifyLevelMax] = "max"
	conf.notifyTranslateReverseMap["max"] = NotifyLevelMax
	//test translation
	t1 := conf.NotifyTranslate(NotifyLevelMax)
	assert.NotEqualf(t, notifyLevelUnknownDescription, t1, "existing value not found")
	t1 = conf.NotifyTranslate(notifyLevelUnknown)
	assert.Equalf(t, notifyLevelUnknownDescription, t1, "non existing value found")
	// test reverse translation
	t2 := conf.NotifyTranslateReverse("max")
	assert.NotEqualf(t, notifyLevelUnknown, t2, "existing value not found")
	t2 = conf.NotifyTranslateReverse(notifyLevelUnknownDescription)
	assert.Equalf(t, notifyLevelUnknown, t2, "non existing value found")
}

// test creation of a new config obj
func TestNewConfig(t *testing.T) {
	if err := os.Setenv("HOME", "test"); err != nil {
		t.Skip("it was not possible to set OS env variable")
	}
	conf := newConfig()
	assert.Equal(t, "test/.liqo", os.Getenv("LIQO_PATH"))
	assert.Equal(t, 3, len(conf.notifyTranslateMap))
	assert.Equal(t, 3, len(conf.notifyTranslateReverseMap))
	// test config startup content
	assert.Equal(t, NotifyLevelMax, conf.NotifyLevel())
	assert.Equal(t, len(conf.NotifyDescriptions()), 3)
}
