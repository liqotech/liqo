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

package consts

const (
	// VirtualNodeTolerationKey all Pods that have to be scheduled on virtual nodes must have this toleration
	// to Liqo taint.
	VirtualNodeTolerationKey = "virtual-node.liqo.io/not-allowed"

	// WebHookLabel used to mark the resouces related to the Liqo webhooks.
	WebHookLabel = "liqo.io/webhook"

	// WebHookLabelValue is the value of the label used to identify Liqo webhooks.
	WebHookLabelValue = "true"
)
