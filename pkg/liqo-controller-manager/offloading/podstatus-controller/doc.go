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

// Package podstatusctrl implements a controller that enforces the presence/absence of the remote unavailable label on
// local offloaded pods. The presence of the label indicates that the status of the offloaded pod can be managed/updated by the local
// cluster and can't currently be reflected/updated from the remote cluster, likely due to a remote cluster failure.
package podstatusctrl
