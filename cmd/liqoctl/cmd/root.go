package cmd

import (
	"context"
	"flag"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

// NewRootCommand initializes the tree of commands.
func NewRootCommand(ctx context.Context) *cobra.Command {
	// rootCmd represents the base command when called without any subcommands.
	var rootCmd = &cobra.Command{
		Use:   "liqoctl",
		Short: common.LiqoctlShortHelp,
		Long:  common.LiqoctlLongHelp,
	}

	// since we cannot access internal klog configuration, we create a new flagset, let klog to install
	// its flags, and we only keep the ones we are intrested in.
	klogFlagset := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(klogFlagset)
	klogFlagset.VisitAll(func(f *flag.Flag) {
		if f.Name == "v" {
			rootCmd.PersistentFlags().AddGoFlag(f)
		}
	})

	rateFlagset := flag.NewFlagSet("rate-limiting", flag.PanicOnError)
	restcfg.InitFlags(rateFlagset)
	rootCmd.PersistentFlags().AddGoFlagSet(rateFlagset)
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable/Disable debug mode (default: false)")
	rootCmd.AddCommand(newInstallCommand(ctx))
	rootCmd.AddCommand(newAddCommand(ctx))
	rootCmd.AddCommand(newGenerateAddCommand(ctx))
	return rootCmd
}
