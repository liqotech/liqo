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

package test

// FakeReflectionManager implements a fake ReflectionManager for testing purposes.
type FakeReflectionManager struct {
	Started map[string]int
	Stopped map[string]int
}

// NewFakeReflectionManager returns a new FakeReflectionManager.
func NewFakeReflectionManager() *FakeReflectionManager {
	return &FakeReflectionManager{
		Started: make(map[string]int),
		Stopped: make(map[string]int),
	}
}

// StartNamespace increments the Started counter for the given namespace.
func (f *FakeReflectionManager) StartNamespace(local, _ string) {
	f.Started[local]++
}

// StopNamespace increments the Stopped counter for the given namespace.
func (f *FakeReflectionManager) StopNamespace(local, _ string) {
	f.Stopped[local]++
}
