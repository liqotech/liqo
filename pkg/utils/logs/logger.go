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

package logs

import (
	"github.com/liqotech/liqo/pkg/liqoctl/common"
)

var printer *common.Printer
var verbose = false

// SetupLogger sets up the logger global variables.
func SetupLogger(p *common.Printer, v bool) {
	printer = p
	verbose = v
}

// Infof logs an info message.
func Infof(format string, args ...interface{}) {
	if printer == nil || !verbose {
		return
	}

	printer.Info.Printf(format, args...)
}
