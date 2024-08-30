// Copyright 2019-2024 The Liqo Authors
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
//

package info

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type dummyChecker struct {
	CheckerBase
	title string
	id    string
	data  interface{}

	nCollectCalls int
}

func (d *dummyChecker) Collect(_ context.Context, _ Options) {
	d.nCollectCalls++
}

func (d *dummyChecker) Format(options Options) string {
	return ""
}

func (d *dummyChecker) GetData() interface{} {
	return d.data
}

// GetID returns the id of the section collected by the checker.
func (d *dummyChecker) GetID() string {
	if d.id != "" {
		return d.id
	}
	return "dummy"
}

// GetTitle returns the title of the section collected by the checker.
func (d *dummyChecker) GetTitle() string {
	if d.id != "" {
		return d.title
	}
	return "Dummy"
}

func TestLocal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Local Suite")
}
