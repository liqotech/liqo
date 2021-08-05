package helm

import (
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
)

// GetChart returns a chart matching the provided chart name and options.
func GetChart(chartName string, chartPathOptions *action.ChartPathOptions, settings *cli.EnvSettings) (*chart.Chart, string, error) {
	chartPath, err := chartPathOptions.LocateChart(chartName, settings)
	if err != nil {
		return nil, "", err
	}

	helmChart, err := loader.Load(chartPath)
	if err != nil {
		return nil, "", err
	}

	return helmChart, chartPath, err
}
