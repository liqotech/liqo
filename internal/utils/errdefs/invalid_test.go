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
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

type testingInvalidInputError bool

func (e testingInvalidInputError) Error() string {
	return fmt.Sprintf("%v", bool(e))
}

func (e testingInvalidInputError) InvalidInput() bool {
	return bool(e)
}

func TestIsInvalidInput(t *testing.T) {
	type testCase struct {
		name          string
		err           error
		xMsg          string
		xInvalidInput bool
	}

	for _, c := range []testCase{
		{
			name:          "InvalidInputf",
			err:           InvalidInputf("%s not found", "foo"),
			xMsg:          "foo not found",
			xInvalidInput: true,
		},
		{
			name:          "AsInvalidInput",
			err:           AsInvalidInput(errors.New("this is a test")),
			xMsg:          "this is a test",
			xInvalidInput: true,
		},
		{
			name:          "AsInvalidInputWithNil",
			err:           AsInvalidInput(nil),
			xMsg:          "",
			xInvalidInput: false,
		},
		{
			name:          "nilError",
			err:           nil,
			xMsg:          "",
			xInvalidInput: false,
		},
		{
			name:          "customInvalidInputFalse",
			err:           testingInvalidInputError(false),
			xMsg:          "false",
			xInvalidInput: false,
		},
		{
			name:          "customInvalidInputTrue",
			err:           testingInvalidInputError(true),
			xMsg:          "true",
			xInvalidInput: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Check(t, cmp.Equal(IsInvalidInput(c.err), c.xInvalidInput))
			if c.err != nil {
				assert.Check(t, cmp.Equal(c.err.Error(), c.xMsg))
			}
		})
	}
}

func TestInvalidInputCause(t *testing.T) {
	err := errors.New("test")
	e := &invalidInputError{err}
	assert.Check(t, cmp.Equal(e.Cause(), err))
	assert.Check(t, IsInvalidInput(errors.Wrap(e, "some details")))
}
