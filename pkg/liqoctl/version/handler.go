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

package version

import (
	"context"
	"fmt"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

var liqoctlVersion = "development"

// Options encapsulates the arguments of the version command.
type Options struct {
	*factory.Factory

	ClientOnly bool
}

// Run implements the version command.
func (o *Options) Run(ctx context.Context) error {
	fmt.Printf("Client version: %s\n", liqoctlVersion)

	if o.ClientOnly {
		return nil
	}

	release, err := o.HelmClient().GetRelease(install.LiqoReleaseName)
	if err != nil {
		o.Printer.Error.Printf("Failed to retrieve release information from namespace %q: %v\n", o.LiqoNamespace, err)
		return err
	}

	if release.Chart == nil || release.Chart.Metadata == nil {
		o.Printer.Error.Print("Invalid release information\n")
		return err
	}

	version := release.Chart.Metadata.AppVersion
	if version == "" {
		// Development version, fallback to the value specified as tag
		tag, ok := release.Config["tag"]
		if !ok {
			o.Printer.Error.Print("Invalid release information\n")
			return err
		}
		version = tag.(string)
	}

	fmt.Printf("Server version: %s\n", version)
	return nil
}
