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

package errdefs

import (
	"errors"
	"fmt"
)

// ErrUnavailable is an error interface which denotes whether the operation failed due
// to the unavailability of a resource.
type ErrUnavailable interface {
	Unavailable() bool
	error
}

type unavailableError struct {
	error
}

func (e *unavailableError) Unavailable() bool {
	return true
}

func (e *unavailableError) Cause() error {
	return e.error
}

// AsUnavailable wraps the passed in error to make it of type ErrUnavailable
//
// Callers should make sure the passed in error has exactly the error message
// it wants as this function does not decorate the message.
func AsUnavailableError(err error) error {
	if err == nil {
		return nil
	}
	return &unavailableError{err}
}

// InvalidInput makes an ErrInvalidInput from the provided error message.
func Unavailable(msg string) error {
	return &unavailableError{errors.New(msg)}
}

// InvalidInputf makes an ErrInvalidInput from the provided error format and args.
func Unavailablef(format string, args ...interface{}) error {
	return &unavailableError{fmt.Errorf(format, args...)}
}

// IsUnavailable determines if the passed in error is of type unavailableError
//
// This will traverse the causal chain (`Cause() error`), until it finds an error
// which implements the `InvalidInput` interface.
func IsUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(ErrUnavailable); ok {
		return e.Unavailable()
	}

	if e, ok := err.(causal); ok {
		return IsUnavailable(e.Cause())
	}

	return false
}
