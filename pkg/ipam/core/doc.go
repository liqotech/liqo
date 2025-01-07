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

// Package ipamcore provides the core functionality for the IPAM service.
//
// The IPAM is organized like a binary tree, where each node represents a network.
// In order to optimize the network allocation we use buddy mmory allocation alghorithm
// to allocate networks like they are memory blocks (https://en.wikipedia.org/wiki/Buddy_memory_allocation).
// When a network is splitted in two, the left child represents the first half of the network, while the right child
// represents the second half. The splitting is done until the network is splitted in blocks of the desired size.
package ipamcore
