// Copyright 2019-2023 The Liqo Authors
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
	"fmt"
	"strings"

	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/peeroob"
	"github.com/liqotech/liqo/pkg/utils"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// Options encapsulates the arguments of the generate peer-command command.
type Options struct {
	*factory.Factory

	CommandName string
	OnlyCommand bool
}

// Run implements the generate peer-command command.
func (o *Options) Run(ctx context.Context) error {
	if o.OnlyCommand {
		command, err := o.generate(ctx)
		if err != nil {
			o.Printer.Error.Printfln("Failed to retrieve peering information: %v", output.PrettyErr(err))
			return err
		}

		fmt.Println(command)
		return nil
	}

	s := o.Printer.StartSpinner("Retrieving peering information")
	command, err := o.generate(ctx)
	if err != nil {
		s.Fail("Failed to retrieve peering information: ", output.PrettyErr(err))
		return err
	}
	s.Success("Peering information correctly retrieved")

	fmt.Printf("\nExecute this command on a *different* cluster to enable an outgoing peering with the current cluster:\n\n")
	fmt.Printf("%s\n", command)
	return nil
}

func (o *Options) generate(ctx context.Context) (string, error) {
	localToken, err := auth.GetToken(ctx, o.CRClient, o.LiqoNamespace)
	if err != nil {
		return "", err
	}

	clusterIdentity, err := utils.GetClusterIdentityWithControllerClient(ctx, o.CRClient, o.LiqoNamespace)
	if err != nil {
		return "", err
	}

	authEP, err := foreigncluster.GetHomeAuthURL(ctx, o.CRClient, o.LiqoNamespace)
	if err != nil {
		return "", err
	}

	// If the local cluster has not a cluster name, we print the use the local clusterID to not leave this field empty.
	// This can be changed by the user when pasting this value in a remote cluster.
	if clusterIdentity.ClusterName == "" {
		clusterIdentity.ClusterName = clusterIdentity.ClusterID
	}

	return strings.Join([]string{
		o.CommandName, "peer out-of-band", clusterIdentity.ClusterName,
		"--" + peeroob.AuthURLFlagName, authEP,
		"--" + peeroob.ClusterIDFlagName, clusterIdentity.ClusterID,
		"--" + peeroob.ClusterTokenFlagName, localToken,
	}, " "), nil
}
