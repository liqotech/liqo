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

package trace

import (
	"time"

	"k8s.io/klog/v2"
)

// LongThreshold returns the treshold to show a tracing log, depending on the configured klog level.
func LongThreshold() time.Duration {
	switch {
	case klog.V(5).Enabled():
		return 100 * time.Millisecond
	case klog.V(4).Enabled():
		return 250 * time.Millisecond
	case klog.V(2).Enabled():
		return 500 * time.Millisecond
	default:
		return time.Second
	}
}
