// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"fmt"
	"time"

	flag "github.com/spf13/pflag"
	"golang.org/x/mod/semver"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/pkg/consts"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

var providersDefaultMTU = map[string]float64{
	"kubeadm":   1440,
	"kind":      1440,
	"k3s":       1440,
	"eks":       1440,
	"gke":       1400,
	"aks":       1360,
	"openshift": 1440,
}

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
	DownloadChart        bool
	ChartTmpDir          string
}

// ValidateCommonArguments validates install common arguments. If the inputs are valid, it returns a *CommonArgument
// with all the parameters contents.
func ValidateCommonArguments(providerName string, flags *flag.FlagSet) (*CommonArguments, error) {
	chartPath, err := flags.GetString("chart-path")
	if err != nil {
		return nil, err
	}
	repoURL, err := flags.GetString("repo-url")
	if err != nil {
		return nil, err
	}
	version, err := flags.GetString("version")
	if err != nil {
		return nil, err
	}
	downloadChart := !flags.Changed("chart-path") && !isRelease(version)
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
	lanDiscovery, err := flags.GetBool(consts.EnableLanDiscoveryParameter)
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
	ifaceMTU, err := flags.GetInt("mtu")
	if err != nil {
		return nil, err
	}
	listeningPort, err := flags.GetInt("vpn-listening-port")
	if err != nil {
		return nil, err
	}
	commonValues, tmpDir, err := parseCommonValues(providerName, &chartPath, repoURL, version,
		resourceSharingPercentage, downloadChart, lanDiscovery, enableHa,
		float64(ifaceMTU), float64(listeningPort))
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
		DownloadChart:        downloadChart,
		ChartTmpDir:          tmpDir,
	}, nil
}

func parseCommonValues(providerName string, chartPath *string, repoURL, version, resourceSharingPercentage string,
	downloadChart, lanDiscovery, enableHa bool,
	mtu, port float64) (values map[string]interface{}, tmpDir string, err error) {
	if chartPath == nil {
		chartPath = pointer.String(installutils.LiqoChartFullName)
	}

	if downloadChart {
		tmpDir, err = cloneRepo(repoURL, version)
		if err != nil {
			return nil, "", err
		}
		fmt.Printf("* Using chart from %s\n", tmpDir)
		*chartPath = fmt.Sprintf("%s/deployments/liqo", tmpDir)
	}

	// If the chartPath is different from the official repo, we force the tag parameter in order to set the correct
	// prefix for the images.
	// (todo): make the prefix configurable and set the tag when is strictly necessary
	tag := ""
	if *chartPath != installutils.LiqoChartFullName {
		tag = version
	}

	resourceSharingPercentageVal := argsutils.Percentage{}
	if err := resourceSharingPercentageVal.Set(resourceSharingPercentage); err != nil {
		return map[string]interface{}{}, "", err
	}

	gatewayReplicas := 1
	if enableHa {
		gatewayReplicas = 2
	}
	if mtu == 0 {
		var err error
		if mtu, err = getDefaultMTU(providerName); err != nil {
			return nil, "", err
		}
	}
	return map[string]interface{}{
		"tag": tag,
		"discovery": map[string]interface{}{
			"config": map[string]interface{}{
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
			"config": map[string]interface{}{
				"listeningPort": port,
			},
		},
		"networkConfig": map[string]interface{}{
			"mtu": mtu,
		},
	}, tmpDir, nil
}

func getDefaultMTU(provider string) (float64, error) {
	if mtu, ok := providersDefaultMTU[provider]; ok {
		return mtu, nil
	}
	return 0, fmt.Errorf("mtu for provider %s not found", provider)
}

func isRelease(version string) bool {
	return version == "" || semver.IsValid(version)
}
