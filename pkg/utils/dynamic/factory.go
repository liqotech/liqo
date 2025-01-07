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

package dynamic

import (
	"context"

	"k8s.io/client-go/dynamic/dynamicinformer"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ manager.Runnable = &RunnableFactory{}

// RunnableFactory is a wrapper around a DynamicSharedInformerFactory to implement the Runnable interface.
type RunnableFactory struct {
	dynamicinformer.DynamicSharedInformerFactory
}

// Start starts the informers.
func (r *RunnableFactory) Start(ctx context.Context) error {
	r.DynamicSharedInformerFactory.Start(ctx.Done())
	r.DynamicSharedInformerFactory.WaitForCacheSync(ctx.Done())
	return nil
}
