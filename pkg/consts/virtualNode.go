// Copyright 2019-2022 The Liqo Authors
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

// NodeFinalizer is the finalizer added on a ResourceOffer when the related VirtualNode is up.
// (managed by the VirtualKubelet).
const NodeFinalizer = "liqo.io/node"

// VirtualKubeletFinalizer is the finalizer added on a ResourceOffer when the related VirtualKubelet is up.
// (managed by the ResourceOffer Operator).
const VirtualKubeletFinalizer = "liqo.io/virtualkubelet"
