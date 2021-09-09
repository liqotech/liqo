// Copyright 2019-2021 The Liqo Authors
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

	"github.com/liqotech/liqo/pkg/liqoctl/common"
	"github.com/liqotech/liqo/pkg/liqoctl/generate"
	installprovider "github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

// HandleInstallCommand implements the "install" command. It detects which provider has to be used, generates the chart
// with provider-specific values. Finally, it performs the installation on the target cluster.
func HandleInstallCommand(ctx context.Context, cmd *cobra.Command, baseCommand, providerName string) {
	config := common.GetLiqoctlRestConfOrDie()
	providerInstance := getProviderInstance(providerName)

	if providerInstance == nil {
		fmt.Printf("Provider of type %s not found", providerName)
		return
	}

	commonArgs, err := installprovider.ValidateCommonArguments(cmd.Flags())
	if err != nil {
		fmt.Printf("Unable to initialize configuration: %v", err)
		os.Exit(1)
	}

	helmClient, err := initHelmClient(config, commonArgs)
	if err != nil {
		fmt.Printf("Unable to create a client for the target cluster: %s", err)
		return
	}

	err = providerInstance.ValidateCommandArguments(cmd.Flags())
	if err != nil {
		fmt.Printf("Unable to initialize configuration: %v", err)
		os.Exit(1)
	}

	err = providerInstance.ExtractChartParameters(ctx, config, commonArgs)
	if err != nil {
		fmt.Printf("Unable to initialize configuration: %v", err)
		os.Exit(1)
	}

	err = installOrUpdate(ctx, helmClient, providerInstance, commonArgs)
	if err != nil {
		fmt.Printf("Unable to initialize configuration: %v", err)
		os.Exit(1)
	}

	if !commonArgs.DumpValues && !commonArgs.DryRun {
		// If the installation succeeded, let's print the add command to peer the target cluster with another one.
		generate.HandleGenerateAddCommand(ctx, installutils.LiqoNamespace, baseCommand)
	}
}
