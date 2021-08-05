package install

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const providerFlag = "provider"

// HandleInstallCommand implements the install command. It detects which provider has to be used, generates the chart
// with provider-specific values. Finally, it performs the installation on the target cluster.
func HandleInstallCommand(cmd *cobra.Command, args []string) {
	config, err := initClientConfig()
	if err != nil {
		fmt.Printf("Unable to create a client for the target cluster: %s", err)
		return
	}
	helmClient, err := initHelmClient(config)
	if err != nil {
		fmt.Printf("Unable to create a client for the target cluster: %s", err)
		return
	}
	ctx := context.Background()
	providerName, err := cmd.Flags().GetString(providerFlag)
	if err != nil {
		return
	}
	provider := getProviderInstance(providerName)

	if provider == nil {
		fmt.Printf("Provider of type %s not found", providerName)
		return
	}

	err = provider.ValidateCommandArguments(cmd.Flags())
	if err != nil {
		fmt.Printf("Unable to initialize configuration: %v", err)
		os.Exit(1)
	}

	err = provider.ExtractChartParameters(ctx, config)
	if err != nil {
		fmt.Printf("Unable to initialize configuration: %v", err)
		os.Exit(1)
	}

	err = installOrUpdate(ctx, helmClient, provider)
	if err != nil {
		fmt.Printf("Unable to initialize configuration: %v", err)
		os.Exit(1)
	}
}
