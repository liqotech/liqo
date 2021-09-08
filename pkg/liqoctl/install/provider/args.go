package provider

import (
	"time"

	flag "github.com/spf13/pflag"

	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// CommonArguments encapsulates all the arguments common across install providers.
type CommonArguments struct {
	Version              string
	Debug                bool
	Timeout              time.Duration
	DumpValues           bool
	DumpValuesPath       string
	DryRun               bool
	CommonValues         map[string]interface{}
	Devel                bool
	DisableEndpointCheck bool
	ChartPath            string
}

// ValidateCommonArguments validates install common arguments. If the inputs are valid, it returns a *CommonArgument
// with all the parameters contents.
func ValidateCommonArguments(flags *flag.FlagSet) (*CommonArguments, error) {
	chartPath, err := flags.GetString("chart-path")
	if err != nil {
		return nil, err
	}
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
	disableEndpointCheck, err := flags.GetBool("disable-endpoint-check")
	if err != nil {
		return nil, err
	}
	resourceSharingPercentage, err := flags.GetString("resource-sharing-percentage")
	if err != nil {
		return nil, err
	}
	enableHa, err := flags.GetBool("enable-ha")
	if err != nil {
		return nil, err
	}
	commonValues, err := parseCommonValues(clusterLabels, chartPath, version, resourceSharingPercentage, lanDiscovery, enableHa)
	if err != nil {
		return nil, err
	}
	return &CommonArguments{
		Version:              version,
		Debug:                debug,
		Timeout:              time.Duration(timeout) * time.Second,
		DryRun:               dryRun,
		DumpValues:           dumpValues,
		DumpValuesPath:       dumpValuesPath,
		CommonValues:         commonValues,
		Devel:                devel,
		DisableEndpointCheck: disableEndpointCheck,
		ChartPath:            chartPath,
	}, nil
}

func parseCommonValues(clusterLabels, chartPath, version, resourceSharingPercentage string,
	lanDiscovery, enableHa bool) (map[string]interface{}, error) {
	clusterLabelsVar := argsutils.StringMap{}
	if err := clusterLabelsVar.Set(clusterLabels); err != nil {
		return map[string]interface{}{}, err
	}

	// If the chartPath is different from the official repo, we force the tag parameter in order to set the correct
	// prefix for the images.
	// (todo): make the prefix configurable and set the tag when is strictly necessary
	tag := ""
	if chartPath != installutils.LiqoChartFullName {
		tag = version
	}

	resourceSharingPercentageVal := argsutils.Percentage{}
	if err := resourceSharingPercentageVal.Set(resourceSharingPercentage); err != nil {
		return map[string]interface{}{}, err
	}

	gatewayReplicas := 1
	if enableHa {
		gatewayReplicas = 2
	}

	return map[string]interface{}{
		"tag": tag,
		"discovery": map[string]interface{}{
			"config": map[string]interface{}{
				"clusterLabels":       installutils.GetInterfaceMap(clusterLabelsVar.StringMap),
				"enableDiscovery":     lanDiscovery,
				"enableAdvertisement": lanDiscovery,
			},
		},
		"controllerManager": map[string]interface{}{
			"config": map[string]interface{}{
				"resourceSharingPercentage": float64(resourceSharingPercentageVal.Val),
			},
		},
		"gateway": map[string]interface{}{
			"replicas": float64(gatewayReplicas),
		},
	}, nil
}
