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

package interfaces

import (
	"context"
	"sync"
)

// UpdaterInterface represents a generic subset of Updater exported methods to be used instead of a direct access to
// a particular Updater instance.
type UpdaterInterface interface {
	// Start runs an instance of an updater which will be stopped when ctx.Done() is closed.
	Start(ctx context.Context, group *sync.WaitGroup)
	// Push adds the clusterID to the internal queue to be processed as soon as possible.
	Push(clusterID string)
}
