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

// NotFound is an error interface which denotes whether the opration failed due
// to a the resource not being found.
type ErrNotFound interface {
	NotFound() bool
	error
}

type notFoundError struct {
	error
}

func (e *notFoundError) NotFound() bool {
	return true
}

func (e *notFoundError) Cause() error {
	return e.error
}

// AsNotFound wraps the passed in error to make it of type ErrNotFound
//
// Callers should make sure the passed in error has exactly the error message
// it wants as this function does not decorate the message.
func AsNotFound(err error) error {
	if err == nil {
		return nil
	}
	return &notFoundError{err}
}

// NotFound makes an ErrNotFound from the provided error message.
func NotFound(msg string) error {
	return &notFoundError{errors.New(msg)}
}

// NotFoundf makes an ErrNotFound from the provided error format and args.
func NotFoundf(format string, args ...interface{}) error {
	return &notFoundError{fmt.Errorf(format, args...)}
}

// IsNotFound determines if the passed in error is of type ErrNotFound
//
// This will traverse the causal chain (`Cause() error`), until it finds an error
// which implements the `NotFound` interface.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(ErrNotFound); ok {
		return e.NotFound()
	}

	if e, ok := err.(causal); ok {
		return IsNotFound(e.Cause())
	}

	return false
}
