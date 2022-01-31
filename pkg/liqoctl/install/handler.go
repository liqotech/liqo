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
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
	"github.com/liqotech/liqo/pkg/liqoctl/generate"
	installprovider "github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
	logsutils "github.com/liqotech/liqo/pkg/utils/logs"
)

// HandleInstallCommand implements the "install" command. It detects which provider has to be used, generates the chart
// with provider-specific values. Finally, it performs the installation on the target cluster.
func HandleInstallCommand(ctx context.Context, cmd *cobra.Command, baseCommand, providerName string) error {
	if !klog.V(4).Enabled() {
		klog.SetLogFilter(logsutils.LogFilter{})
	}

	config, err := common.GetLiqoctlRestConf()
	if err != nil {
		return err
	}
	providerInstance := getProviderInstance(providerName)

	if providerInstance == nil {
		return fmt.Errorf("provider %s not found", providerName)
	}

	fmt.Printf("* Initializing installer... 🔌 \n")
	commonArgs, err := installprovider.ValidateCommonArguments(providerName, cmd.Flags())
	if err != nil {
		return err
	}

	if commonArgs.DownloadChart {
		defer os.RemoveAll(commonArgs.ChartTmpDir)
	}

	helmClient, err := initHelmClient(config, commonArgs)
	if err != nil {
		return err
	}

	oldClusterName, err := installutils.GetOldClusterName(ctx, kubernetes.NewForConfigOrDie(config))
	if err != nil {
		return err
	}

	fmt.Printf("* Retrieving cluster configuration from cluster provider... 📜  \n")
	err = providerInstance.PreValidateGenericCommandArguments(cmd.Flags())
	if err != nil {
		return err
	}

	err = providerInstance.ValidateCommandArguments(cmd.Flags())
	if err != nil {
		return err
	}

	err = providerInstance.PostValidateGenericCommandArguments(oldClusterName)
	if err != nil {
		return err
	}

	err = providerInstance.ExtractChartParameters(ctx, config, commonArgs)
	if err != nil {
		return err
	}

	if commonArgs.DumpValues {
		fmt.Printf("* Generating values.yaml file with the Liqo chart parameters for your cluster... 💾 \n")
	} else {
		fmt.Printf("* Installing or Upgrading Liqo... (this may take few minutes) ⏳ \n")
	}
	err = installOrUpdate(ctx, helmClient, providerInstance, commonArgs)
	if err != nil {
		return err
	}

	switch {
	case !commonArgs.DumpValues && !commonArgs.DryRun:
		fmt.Printf("* All Set! You can use Liqo now! 🚀\n")
		return generate.HandleGenerateAddCommand(ctx, installutils.LiqoNamespace, false, baseCommand)
	case commonArgs.DumpValues:
		fmt.Printf("* All Set! Chart values written in file %s 🚀\n", commonArgs.DumpValuesPath)
	case commonArgs.DryRun:
		fmt.Printf("* All Set! You can use Liqo now! (Dry-run) 🚀\n")
	}
	return nil
}
