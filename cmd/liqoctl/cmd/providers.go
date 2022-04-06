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

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/install"
	"github.com/liqotech/liqo/pkg/liqoctl/install/aks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/eks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/gke"
	"github.com/liqotech/liqo/pkg/liqoctl/install/k3s"
	"github.com/liqotech/liqo/pkg/liqoctl/install/kubeadm"
	"github.com/liqotech/liqo/pkg/liqoctl/install/openshift"
)

const (
	installShortHelp = "Install Liqo on a selected %s cluster"
	installLongHelp  = "Install Liqo on a selected %s cluster"
)

var providerInitFunc = map[string]func(*cobra.Command){
	"kubeadm":   kubeadm.GenerateFlags,
	"kind":      kubeadm.GenerateFlags,
	"k3s":       k3s.GenerateFlags,
	"eks":       eks.GenerateFlags,
	"gke":       gke.GenerateFlags,
	"aks":       aks.GenerateFlags,
	"openshift": openshift.GenerateFlags,
}

func getCommand(ctx context.Context, provider string) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:          provider,
		Short:        fmt.Sprintf(installShortHelp, provider),
		Long:         fmt.Sprintf(installLongHelp, provider),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return install.HandleInstallCommand(ctx, cmd, os.Args[0], provider)
		},
	}

	initFunc, ok := providerInitFunc[provider]
	if !ok {
		return nil, fmt.Errorf("initFunc not found for provider %s not found", provider)
	}

	initFunc(cmd)

	return cmd, nil
}
