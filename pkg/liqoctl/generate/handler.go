package generate

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/add"
	"github.com/liqotech/liqo/pkg/liqoctl/common"
	"github.com/liqotech/liqo/pkg/utils"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// HandleGenerateAddCommand outputs the liqoctl add command to use to add the target cluster.
func HandleGenerateAddCommand(ctx context.Context, liqoNamespace, commandName string) {
	restConfig := common.GetLiqoctlRestConfOrDie()

	clientSet, err := client.New(restConfig, client.Options{})
	if err != nil {
		klog.Fatalf(err.Error())
	}

	commandString := processGenerateCommand(ctx, clientSet, liqoNamespace, commandName)

	fmt.Printf("Use this command to peer with this cluster:\n\n")
	fmt.Printf("%s\n", commandString)
}

func processGenerateCommand(ctx context.Context, clientSet client.Client, liqoNamespace, commandName string) string {
	localToken, err := auth.GetToken(ctx, clientSet, liqoNamespace)
	if err != nil {
		klog.Fatalf(err.Error())
	}

	clusterID, err := utils.GetClusterIDWithControllerClient(ctx, clientSet, liqoNamespace)
	if err != nil {
		klog.Fatalf(err.Error())
	}

	clusterConfig := &configv1alpha1.ClusterConfig{}
	err = clientSet.Get(ctx, types.NamespacedName{
		Name: consts.ClusterConfigResourceName,
	}, clusterConfig)
	if err != nil {
		klog.Fatalf("an error occurred while retrieving the clusterConfig: %s", err)
	}

	authEP, err := foreigncluster.GetHomeAuthURL(ctx, clientSet, clientSet,
		clusterConfig.Spec.DiscoveryConfig.AuthServiceAddress, clusterConfig.Spec.DiscoveryConfig.AuthServicePort, liqoNamespace)
	if err != nil {
		klog.Fatalf("an error occurred while retrieving the liqo-auth service: %s", err)
	}
	return generateCommandString(commandName, authEP, clusterID, localToken, clusterConfig.Spec.DiscoveryConfig.ClusterName)
}

func generateCommandString(commandName, authEP, clusterID, localToken, clusterName string) string {
	// If the local cluster has not clusterName, we print the local clusterID to not leave this field empty.
	// This can be changed bt the user when pasting this value in a remote cluster.
	if clusterName == "" {
		clusterName = clusterID
	}

	command := []string{commandName,
		add.UseCommand,
		add.ClusterResourceName,
		clusterName,
		"--" + add.AuthURLFlagName,
		authEP,
		"--" + add.ClusterIDFlagName,
		clusterID,
		"--" + add.ClusterTokenFlagName,
		localToken}
	return strings.Join(command, " ")
}
