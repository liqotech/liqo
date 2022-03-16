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

package uninstall

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/storage/driver"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

// HandleUninstallCommand implements the "uninstall" command.
func HandleUninstallCommand(ctx context.Context, cmd *cobra.Command, args *Args) error {
	printer := common.NewPrinter("", common.Cluster1Color)

	s, err := printer.Spinner.Start("Loading configuration")
	utilruntime.Must(err)

	config, err := common.GetLiqoctlRestConf()
	if err != nil {
		s.Fail("Error loading configuration: ", err)
		return err
	}

	helmClient, err := initHelmClient(config, args.Namespace)
	if err != nil {
		s.Fail("Error initializing helm client: ", err)
		return err
	}
	s.Success("Configuration loaded")

	s, err = printer.Spinner.Start("Uninstalling Liqo")
	utilruntime.Must(err)

	err = helmClient.UninstallReleaseByName(installutils.LiqoReleaseName)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		s.Fail("Error uninstalling Liqo: ", err)
		return err
	}
	s.Success("Liqo uninstalled")

	if args.Purge {
		s, err = printer.Spinner.Start("Purging Liqo CRDs")
		utilruntime.Must(err)

		if err = purge(ctx, config); err != nil {
			s.Fail("Error purging CRDs: ", err)
			return err
		}
		s.Success("Liqo CRDs purged")
	}

	return nil
}
