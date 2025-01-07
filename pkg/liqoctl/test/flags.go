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

package test

import (
	"time"

	"github.com/spf13/pflag"
)

// FlagNames contains the names of the flags.
type FlagNames string

const (
	// FlagNamesTimeout is the flag name for the timeout.
	FlagNamesTimeout FlagNames = "timeout"
	// FlagNamesVerbose is the flag name for the verbose output.
	FlagNamesVerbose FlagNames = "verbose"
	// FlagNamesFailFast is the flag name for the fail-fast option.
	FlagNamesFailFast FlagNames = "fail-fast"
)

// AddFlags adds the flags to the flag set.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.DurationVar(&o.Timeout, string(FlagNamesTimeout), 5*time.Minute, "Timeout for the test")
	fs.BoolVarP(&o.Verbose, string(FlagNamesVerbose), "v", false, "Verbose output")
	fs.BoolVar(&o.FailFast, string(FlagNamesFailFast), false, "Stop the test as soon as an error is encountered")
}
