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

package liqonodeprovider

import (
	"context"
	"encoding/json"

	"gomodules.xyz/jsonpatch/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// patchNode patches the controlled node applying the provided function.
func (p *LiqoNodeProvider) patchNode(ctx context.Context, changeFunc func(node *v1.Node) error) error {
	original, err := json.Marshal(p.node)
	if err != nil {
		klog.Error(err)
		return err
	}

	newNode := p.node.DeepCopy()
	err = changeFunc(newNode)
	if err != nil {
		klog.Error(err)
		return err
	}

	target, err := json.Marshal(newNode)
	if err != nil {
		klog.Error(err)
		return err
	}

	ops, err := jsonpatch.CreatePatch(original, target)
	if err != nil {
		klog.Error(err)
		return err
	}

	if len(ops) == 0 {
		// this avoids an empty patch of the node
		p.node = newNode
		return nil
	}

	bytes, err := json.Marshal(ops)
	if err != nil {
		klog.Error(err)
		return err
	}

	p.node, err = p.localClient.CoreV1().Nodes().Patch(ctx,
		p.node.Name, types.JSONPatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
}
