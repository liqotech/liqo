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

package errors

import (
	"strings"

	"github.com/spf13/pflag"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/klog/v2"
)

var panicOnErrorMode = false

// InitFlags initializes the flags to configure the errormanagement parameter.
func InitFlags(flagset *pflag.FlagSet) {
	if flagset == nil {
		flagset = pflag.CommandLine
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
		}
		klog.Errorf("%s", err)
		return false
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

// IgnoreNoMatchError returns nil on NoMatch errors.
// All other values that are not NoMatch errors or nil are returned unmodified.
func IgnoreNoMatchError(err error) error {
	if meta.IsNoMatchError(err) {
		return nil
	}
	return err
}

// CheckFakeClientServerSideApplyError check if an error is due to the fake client not supporting server-side apply.
// Warning: it should only be used as a workaround to skip tests for error stemming from fake k8s client, which should
// be revisited once dependencies are upgraded: "apply patches are not supported in the fake client.
// Follow https://github.com/kubernetes/kubernetes/issues/115598 for the current status".
func CheckFakeClientServerSideApplyError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "apply patches are not supported in the fake client")
}
