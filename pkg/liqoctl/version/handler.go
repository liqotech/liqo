// Copyright 2019-2025 The Liqo Authors
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
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
)

// LiqoctlVersion is the version of the Liqo CLI. It is set at build time.
var LiqoctlVersion = "unknown"

// Options encapsulates the arguments of the version command.
type Options struct {
	*factory.Factory

	ClientOnly bool
}

// Run implements the version command.
func (o *Options) Run(ctx context.Context) error {
	fmt.Printf("Client version: %s\n", LiqoctlVersion)

	if o.ClientOnly {
		return nil
	}

	serverVersion, err := liqogetters.GetLiqoVersion(ctx, o.CRClient, o.LiqoNamespace)
	if err != nil {
		fmt.Println("Server version: Unknown")
		o.Printer.Warning.Printfln("Failed to retrieve Liqo server version: %v", output.PrettyErr(err))
		o.Printer.Warning.Println(
			"Is Liqo installed in your cluster? Is the cluster reachable? Do you have the permissions to access the target cluster?")
		return err
	}

	fmt.Printf("Server version: %s\n", serverVersion)

	if serverVersion != LiqoctlVersion {
		o.Printer.Warning.Println("The version of liqoctl does not match the Liqo server version. This might cause unexpected behaviors.")
	}

	return nil
}
