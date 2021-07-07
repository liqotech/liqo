// Copyright 2019-2021 The Liqo Authors
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

// Package testutils encapsulates all methods and constants to perform E2E tests
package testutils

import "github.com/liqotech/liqo/pkg/consts"

const (
	liqoTestingLabelKey = "liqo.io/testing-namespace"
)

// LiqoTestNamespaceLabels is a set of labels that has to be attached to test namespaces to simplify garbage collection.
var LiqoTestNamespaceLabels = map[string]string{
	liqoTestingLabelKey:      "true",
	consts.EnablingLiqoLabel: consts.EnablingLiqoLabelValue,
}
