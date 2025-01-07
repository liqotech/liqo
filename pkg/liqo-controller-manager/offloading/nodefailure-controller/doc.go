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

// Package nodefailurectrl contains a controller that enforces a logic that
// ensure offloaded pods running on a failed node are evicted and rescheduled
// on a healthy node, preventing them to remain in a terminating state indefinitely.
// This feature can be useful in case of remote node failure to guarantee better
// service continuity and to have the expected pods workload on the remote cluster
package nodefailurectrl
