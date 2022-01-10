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

package main

import (
	"os"

	"github.com/liqotech/liqo/pkg/mutate"
)

const (
	defaultCertFile = "/etc/ssl/liqo/tls.crt"
	defaultKeyFile  = "/etc/ssl/liqo/tls.key"
)

func setOptions(c *mutate.MutationConfig) {
	if c.KeyFile = os.Getenv("LIQO_KEY"); c.KeyFile == "" {
		c.KeyFile = defaultKeyFile
	}

	if c.CertFile = os.Getenv("LIQO_CERT"); c.CertFile == "" {
		c.CertFile = defaultCertFile
	}
}
