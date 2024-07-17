// Copyright 2019-2024 The Liqo Authors
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

package fabric

import (
	"github.com/spf13/pflag"
)

// FlagName is the type for the name of the flags.
type FlagName string

func (fn FlagName) String() string {
	return string(fn)
}

const (
	// FlagNameDisableARP is the flag to enable ARP.
	FlagNameDisableARP FlagName = "disable-arp"
)

// InitFlags initializes the flags for the gateway.
func InitFlags(flagset *pflag.FlagSet, opts *Options) {
	flagset.BoolVar(&opts.DisableARP, FlagNameDisableARP.String(), false, "Disable ARP")
}
