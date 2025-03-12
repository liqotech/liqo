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

// package main contains the entrypoint of the program, which generates the markdown documentation for the liqoctl command.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/liqotech/liqo/cmd/liqoctl/cmd"
	"github.com/liqotech/liqo/hack/liqoctl-docs-generator/pkg/docs"
)

func main() {
	var outputPath string

	flag.StringVar(&outputPath, "o", "", "The output path for the generated documentation")
	flag.Parse()

	if outputPath == "" {
		fmt.Println("output path must be provided")
		os.Exit(1)
	}

	liqoctl := cmd.NewRootCommand(context.TODO())
	liqoctl.Use = "liqoctl"

	err := docs.GenCommandPage(liqoctl, outputPath, true, 0)
	if err != nil {
		panic(err)
	}

	fmt.Println("📖 Documentation generated successfully")
}
