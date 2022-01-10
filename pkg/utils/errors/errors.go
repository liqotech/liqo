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

package errors

import (
	"flag"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
)

var panicOnErrorMode = false

// InitFlags initializes the flags to configure the errormanagement parameter.
func InitFlags(flagset *flag.FlagSet) {
	if flagset == nil {
		flagset = flag.CommandLine
	}

	flagset.BoolVar(&panicOnErrorMode, "panic-on-unexpected-errors", panicOnErrorMode,
		"Enable a pedantic mode which causes a panic if an unexpected error occurs")
}

// SetPanicOnErrorMode can be used to set or unset the panic mode.
func SetPanicOnErrorMode(status bool) {
	panicOnErrorMode = status
}

// Must wraps a function call that can return an error. If some error occurred Must has two possible behaviors:
// panic if debug = true or log the error and return false in order to recover the error.
// Returns true if no error occurred.
func Must(err error) bool {
	if err != nil {
		if panicOnErrorMode {
			panic(err)
		} else {
			klog.Errorf("%s", err)
			return false
		}
	}
	return true
}

// IgnoreAlreadyExists returns nil on AlreadyExists errors.
// All other values that are not AlreadyExists errors or nil are returned unmodified.
func IgnoreAlreadyExists(err error) error {
	if kerrors.IsAlreadyExists(err) {
		return nil
	}

	return err
}
