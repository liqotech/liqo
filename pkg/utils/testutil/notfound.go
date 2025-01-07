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

package testutil

import (
	"fmt"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/api/errors"
)

// IsNotFoundErrorMatcher is a custom matcher to check when kubernetes resources do not exist.
type IsNotFoundErrorMatcher struct{}

// FailBecauseNotFound returns a new IsNotFoundErrorMatcher to catch k8s not-found errors.
func FailBecauseNotFound() types.GomegaMatcher {
	return &IsNotFoundErrorMatcher{}
}

// BeNotFound returns a new IsNotFoundErrorMatcher to catch k8s not-found errors.
func BeNotFound() types.GomegaMatcher {
	return &IsNotFoundErrorMatcher{}
}

// Match is a GomegaMatcher interface method to actually run the matcher.
func (s *IsNotFoundErrorMatcher) Match(actual interface{}) (success bool, err error) {
	if actual == nil {
		return false, nil
	}

	switch e := actual.(type) {
	case error:
		return errors.IsNotFound(e), nil
	default:
		return false, fmt.Errorf("IsNotFoundErrorMatcher can match errors only")
	}
}

// FailureMessage is called when the matcher fails positively.
func (s *IsNotFoundErrorMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to match NotFoundError")
}

// NegatedFailureMessage is called when the matcher fails negatively.
func (s *IsNotFoundErrorMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to match NotFoundError")
}
