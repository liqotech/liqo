// Copyright 2019-2026 The Liqo Authors
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

package connection

import (
	"github.com/spf13/pflag"
)

// FlagName is the type for the name of the flags.
type FlagName string

func (fn FlagName) String() string {
	return string(fn)
}

const (
	// EnableConnectionControllerFlag is the name of the flag used to enable the connection controller.
	EnableConnectionControllerFlag FlagName = "enable-connection-controller"
)

// InitFlags initializes the flags for the wireguard tunnel.
func InitFlags(flagset *pflag.FlagSet, options *Options) {
	flagset.BoolVar(&options.EnableConnectionController, EnableConnectionControllerFlag.String(), true,
		"enable-connection-controller enables the connection controller. It is useful if the tunnel technology implements a connection check.")
}
