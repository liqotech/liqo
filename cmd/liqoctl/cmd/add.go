package cmd

import (
	"context"

	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/liqoctl/add"
)

// installCmd represents the generateInstall command.
func newAddCommand(ctx context.Context) *cobra.Command {
	var addCmd = &cobra.Command{
		Use: add.UseCommand,
	}
	addCmd.AddCommand(newAddClusterCommand(ctx))
	return addCmd
}

func newAddClusterCommand(ctx context.Context) *cobra.Command {
	installArgs := &add.ClusterArgs{}
	var addClusterCmd = &cobra.Command{
		Use:   add.ClusterResourceName,
		Short: add.LiqoctlAddShortHelp,
		Long:  add.LiqoctlAddLongHelp,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			installArgs.ClusterName = args[0]
			add.HandleAddCommand(ctx, installArgs)
		},
	}
	addClusterCmd.Flags().StringVar(&installArgs.ClusterAuthURL, add.AuthURLFlagName, "",
		"the AuthURL of the target Foreign Cluster")
	addClusterCmd.Flags().StringVar(&installArgs.ClusterToken, add.ClusterTokenFlagName, "",
		"the ClusterToken of the target Foreign Cluster")
	addClusterCmd.Flags().StringVar(&installArgs.ClusterID, add.ClusterIDFlagName, "",
		"the ClusterID assigned of the target Foreign Cluster")
	addClusterCmd.Flags().StringVar(&installArgs.Namespace, add.ClusterLiqoNamespaceFlagName, add.ClusterLiqoNamespace,
		"the namespace where Liqo is installed on the cluster")
	utilruntime.Must(addClusterCmd.MarkFlagRequired(add.ClusterIDFlagName))
	utilruntime.Must(addClusterCmd.MarkFlagRequired(add.ClusterTokenFlagName))
	utilruntime.Must(addClusterCmd.MarkFlagRequired(add.AuthURLFlagName))
	return addClusterCmd
}
