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

package generate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/consts"
	liqocontrollermanager "github.com/liqotech/liqo/pkg/liqo-controller-manager"
	"github.com/liqotech/liqo/pkg/liqoctl/add"
	"github.com/liqotech/liqo/pkg/liqoctl/common"
	"github.com/liqotech/liqo/pkg/utils"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// HandleGenerateAddCommand outputs the liqoctl add command to use to add the target cluster.
func HandleGenerateAddCommand(ctx context.Context, liqoNamespace string, printOnlyCommand bool, commandName string) error {
	restConfig, err := common.GetLiqoctlRestConf()
	if err != nil {
		print(liqoctlGenerateRemindInstall)
		return err
	}

	clientSet, err := client.New(restConfig, client.Options{})
	if err != nil {
		print(liqoctlGenerateRemindInstall)
		return err
	}

	commandString, err := processGenerateCommand(ctx, clientSet, liqoNamespace, commandName)
	if err != nil {
		print(liqoctlGenerateRemindInstall)
		return err
	}

	if printOnlyCommand {
		fmt.Println(commandString)
	} else {
		fmt.Printf("\nUse this command on a DIFFERENT cluster to enable an outgoing peering WITH THE CURRENT cluster 🛠:\n\n")
		fmt.Printf("%s\n\n", commandString)
	}
	return nil
}

func processGenerateCommand(ctx context.Context, clientSet client.Client, liqoNamespace, commandName string) (string, error) {
	localToken, err := auth.GetToken(ctx, clientSet, liqoNamespace)
	if err != nil {
		return "", err
	}

	clusterIdentity, err := utils.GetClusterIdentityWithControllerClient(ctx, clientSet, liqoNamespace)
	if err != nil {
		return "", err
	}
	clusterID := clusterIdentity.ClusterID

	// Retrieve the liqo controller manager deployment args
	args, err := RetrieveLiqoControllerManagerDeploymentArgs(ctx, clientSet, liqoNamespace)
	if err != nil {
		return "", err
	}

	// The error is discarded, since an empty string is returned in case the key is not found, which is fine.
	clusterName, _ := common.ExtractValueFromArgumentList(fmt.Sprintf("--%v", consts.ClusterNameParameter), args)
	authServiceAddressOverride, _ := common.ExtractValueFromArgumentList(fmt.Sprintf("--%v", consts.AuthServiceAddressOverrideParameter), args)
	authServicePortOverride, _ := common.ExtractValueFromArgumentList(fmt.Sprintf("--%v", consts.AuthServicePortOverrideParameter), args)

	authEP, err := foreigncluster.GetHomeAuthURL(ctx, clientSet,
		authServiceAddressOverride, authServicePortOverride, liqoNamespace)
	if err != nil {
		return "", err
	}
	return generateCommandString(commandName, authEP, clusterID, localToken, clusterName), nil
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

// RetrieveLiqoControllerManagerDeploymentArgs retrieves the list of arguments associated with the liqo controller manager deployment.
func RetrieveLiqoControllerManagerDeploymentArgs(ctx context.Context, clientSet client.Client, liqoNamespace string) ([]string, error) {
	// Retrieve the deployment of the liqo controller manager component
	var deployments appsv1.DeploymentList
	if err := clientSet.List(ctx, &deployments, client.InNamespace(liqoNamespace), client.MatchingLabelsSelector{
		Selector: liqocontrollermanager.DeploymentLabelSelector(),
	}); err != nil || len(deployments.Items) != 1 {
		return nil, errors.New("failed to retrieve the liqo controller manager deployment")
	}

	containers := deployments.Items[0].Spec.Template.Spec.Containers
	if len(containers) != 1 {
		return nil, errors.New("retrieved an invalid liqo controller manager deployment")
	}

	return containers[0].Args, nil
}
