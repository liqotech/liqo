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

package install

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
	"github.com/liqotech/liqo/pkg/liqoctl/generate"
	installprovider "github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

// HandleInstallCommand implements the "install" command. It detects which provider has to be used, generates the chart
// with provider-specific values. Finally, it performs the installation on the target cluster.
func HandleInstallCommand(ctx context.Context, cmd *cobra.Command, baseCommand, providerName string) error {
	printer := common.NewPrinter("", common.Cluster1Color)

	s, err := printer.Spinner.Start("Loading configuration")
	utilruntime.Must(err)

	config, err := common.GetLiqoctlRestConf()
	if err != nil {
		s.Fail("Error loading configuration: ", err)
		return err
	}

	providerInstance := getProviderInstance(providerName)
	if providerInstance == nil {
		err = fmt.Errorf("provider %s not supported", providerName)
		s.Fail("Error loading configuration: ", err)
		return err
	}
	s.Success("Configuration loaded")

	s, err = printer.Spinner.Start("Initializing installer")
	utilruntime.Must(err)

	commonArgs, err := installprovider.ValidateCommonArguments(providerName, cmd.Flags(), s)
	if err != nil {
		s.Fail("Error initializing installer: ", err)
		return err
	}

	if commonArgs.DownloadChart {
		defer os.RemoveAll(commonArgs.ChartTmpDir)
	}

	helmClient, err := initHelmClient(config, commonArgs)
	if err != nil {
		s.Fail("Error initializing installer: ", err)
		return err
	}

	oldClusterName, err := installutils.GetOldClusterName(ctx, kubernetes.NewForConfigOrDie(config))
	if err != nil {
		s.Fail("Error initializing installer: ", err)
		return err
	}
	s.Success("Installer initialized")

	s, err = printer.Spinner.Start("Retrieving cluster configuration from cluster provider")
	utilruntime.Must(err)

	err = providerInstance.PreValidateGenericCommandArguments(cmd.Flags())
	if err != nil {
		s.Fail("Error validating generic arguments: ", err)
		return err
	}

	err = providerInstance.ValidateCommandArguments(cmd.Flags())
	if err != nil {
		s.Fail("Error validating command arguments: ", err)
		return err
	}

	err = providerInstance.PostValidateGenericCommandArguments(oldClusterName)
	if err != nil {
		s.Fail("Error validating generic arguments: ", err)
		return err
	}

	err = providerInstance.ExtractChartParameters(ctx, config, commonArgs)
	if err != nil {
		s.Fail("Error extracting chart parameters: ", err)
		return err
	}
	s.Success("Chart parameters extracted")

	if commonArgs.DumpValues {
		s, err = printer.Spinner.Start("Generating values.yaml file with the Liqo chart parameters for your cluster")
	} else {
		s, err = printer.Spinner.Start("Installing or Upgrading Liqo... (this may take few minutes)")
	}
	utilruntime.Must(err)

	err = installOrUpdate(ctx, helmClient, providerInstance, commonArgs)
	if err != nil {
		s.Fail("Error installing or upgrading Liqo: ", err)
		return err
	}

	switch {
	case !commonArgs.DumpValues && !commonArgs.DryRun:
		s.Success("All Set! You can use Liqo now!")
		return generate.HandleGenerateAddCommand(ctx, installutils.LiqoNamespace, false, baseCommand)
	case commonArgs.DumpValues:
		s.Success(fmt.Sprintf("All Set! Chart values written in file %s", commonArgs.DumpValuesPath))
	case commonArgs.DryRun:
		s.Success("All Set! You can use Liqo now! (Dry-run)")
	}
	return nil
}
