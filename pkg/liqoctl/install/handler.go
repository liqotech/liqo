package install

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
)

const providerFlag = "provider"

// HandleInstallCommand implements the install command. It detects which provider has to be used, generates the chart
// with provider-specific values. Finally, it performs the installation on the target cluster.
func HandleInstallCommand(ctx context.Context, cmd *cobra.Command, args []string) {
	config, err := initClientConfig()
	if err != nil {
		fmt.Printf("Unable to create a client for the target cluster: %s", err)
		return
	}

	providerName, err := cmd.Flags().GetString(providerFlag)
	if err != nil {
		return
	}
	provider := getProviderInstance(providerName)

	if provider == nil {
		fmt.Printf("Provider of type %s not found", providerName)
		return
	}

	commonArgs, err := ValidateCommonArguments(cmd.Flags())
	if err != nil {
		fmt.Printf("Unable to initialize configuration: %v", err)
		os.Exit(1)
	}

	helmClient, err := initHelmClient(config, commonArgs)
	if err != nil {
		fmt.Printf("Unable to create a client for the target cluster: %s", err)
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

	err = installOrUpdate(ctx, helmClient, provider, commonArgs)
	if err != nil {
		fmt.Printf("Unable to initialize configuration: %v", err)
		os.Exit(1)
	}
}

func ValidateCommonArguments(flags *flag.FlagSet) (*provider.CommonArguments, error) {
	version, err := flags.GetString("version")
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
	return &provider.CommonArguments{
		Version: version,
		Debug:   debug,
		Timeout: time.Duration(timeout) * time.Second,
	}, nil
}
