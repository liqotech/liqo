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

package geneve

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// FlagName is the type for the name of the flags.
type FlagName string

func (fn FlagName) String() string {
	return string(fn)
}

const (
	// FlagNameGeneveID is the interval between two DNS checks.
	FlagNameGeneveID FlagName = "geneve-id"
)

// RequiredFlags contains the list of the mandatory flags.
var RequiredFlags = []FlagName{
	FlagNameGeneveID,
}

// InitFlags initializes the flags for the wireguard tunnel.
func InitFlags(flagset *pflag.FlagSet, opts *Options) {
	flagset.Uint32Var(&opts.GeneveID, FlagNameGeneveID.String(), 10, "Geneve ID")
}

// MarkFlagsRequired marks the flags as required.
func MarkFlagsRequired(cmd *cobra.Command) error {
	for _, flag := range RequiredFlags {
		if err := cmd.MarkFlagRequired(flag.String()); err != nil {
			return err
		}
	}
	return nil
}
