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

package main

import (
	"context"
	"flag"
	"os"

	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/proxy"
)

func main() {
	ctx := context.Background()

	port := flag.Int("port", 8080, "port to listen on")
	allowedHosts := flag.String("allowed-hosts", "", "comma separated list of allowed hosts")
	forceHost := flag.String("force-host", "", "force the server Host to this value")

	flag.Parse()

	p := proxy.New(*allowedHosts, *port, *forceHost)

	if err := p.Start(ctx); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}
