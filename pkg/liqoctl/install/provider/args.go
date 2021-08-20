package provider

import (
	"fmt"
	"strings"
	"time"

	flag "github.com/spf13/pflag"
)

// CommonArguments encapsulates all the arguments common across install providers.
type CommonArguments struct {
	Version        string
	Debug          bool
	Timeout        time.Duration
	DumpValues     bool
	DumpValuesPath string
	DryRun         bool
	CommonValues   map[string]interface{}
	Devel          bool
}

// ValidateCommonArguments validates install common arguments. If the inputs are valid, it returns a *CommonArgument
// with all the parameters contents.
func ValidateCommonArguments(flags *flag.FlagSet) (*CommonArguments, error) {
	version, err := flags.GetString("version")
	if err != nil {
		return nil, err
	}
	devel, err := flags.GetBool("devel")
	if err != nil {
		return nil, err
	}
	debug, err := flags.GetBool("debug")
	if err != nil {
		return nil, err
	}
	timeout, err := flags.GetInt("timeout")
	if err != nil {
		return nil, err
	}
	dryRun, err := flags.GetBool("dry-run")
	if err != nil {
		return nil, err
	}
	dumpValues, err := flags.GetBool("only-output-values")
	if err != nil {
		return nil, err
	}
	dumpValuesPath, err := flags.GetString("dump-values-path")
	if err != nil {
		return nil, err
	}
	clusterLabels, err := flags.GetString("cluster-labels")
	if err != nil {
		return nil, err
	}
	lanDiscovery, err := flags.GetBool("enable-lan-discovery")
	if err != nil {
		return nil, err
	}
	commonValues, err := parseCommonValues(clusterLabels, lanDiscovery)
	if err != nil {
		return nil, err
	}
	return &CommonArguments{
		Version:        version,
		Debug:          debug,
		Timeout:        time.Duration(timeout) * time.Second,
		DryRun:         dryRun,
		DumpValues:     dumpValues,
		DumpValuesPath: dumpValuesPath,
		CommonValues:   commonValues,
		Devel:          devel,
	}, nil
}

func parseCommonValues(clusterLabels string, lanDiscovery bool) (map[string]interface{}, error) {
	if clusterLabels == "" {
		return map[string]interface{}{}, nil
	}
	parts := strings.Split(strings.Trim(clusterLabels, " "), ",")
	m := make(map[string]interface{}, len(parts))
	for i := range parts {
		label := strings.Split(parts[i], "=")
		if len(label) != 2 {
			return map[string]interface{}{}, fmt.Errorf("label string parsing error")
		}
		m[label[0]] = label[1]
	}
	return map[string]interface{}{
		"discovery": map[string]interface{}{
			"config": map[string]interface{}{
				"clusterLabels":       m,
				"enableDiscovery":     lanDiscovery,
				"enableAdvertisement": lanDiscovery,
			},
		},
	}, nil
}
