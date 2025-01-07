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

package flags

import (
	"flag"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

// InitKlogFlags initializes the klog flags.
func InitKlogFlags(flags *pflag.FlagSet) {
	if flags == nil {
		flags = pflag.CommandLine
	}

	legacyflags := flag.NewFlagSet("legacy", flag.ExitOnError)
	klog.InitFlags(legacyflags)
	legacyflags.VisitAll(func(f *flag.Flag) {
		f.Name = "klog." + f.Name
		flags.AddGoFlag(f)
	})
}
