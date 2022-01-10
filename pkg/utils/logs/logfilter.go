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
	"fmt"
	"strings"
)

// LogFilter implements the klog LogFilter interface, to filter log messages in the liqoctl install command.
type LogFilter struct{}

// Filter filters the helm CRDs installation errors.
func (l LogFilter) Filter(args []interface{}) []interface{} {
	if len(args) == 0 {
		return args
	}

	err, ok := args[0].(error)
	if !ok {
		return args
	}

	if strings.
		Contains(err.Error(), "couldn't get resource list for") && strings.
		Contains(err.Error(), "the server could not find the requested resource") {
		// since we cannot cancel the log, we change the error message to "continuing"
		return []interface{}{
			fmt.Errorf("continuing"),
		}
	}
	return args
}

// FilterF does nothing (i.e, only passes through the arguments), as not used in this context.
func (l LogFilter) FilterF(format string, args []interface{}) (f string, a []interface{}) {
	return format, args
}

// FilterS does nothing (i.e, only passes through the arguments), as not used in this context.
func (l LogFilter) FilterS(msg string, keysAndValues []interface{}) (m string, a []interface{}) {
	return msg, keysAndValues
}
