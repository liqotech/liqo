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
func (p *LiqoNodeProvider) patchNode(changeFunc func(node *v1.Node) error) error {
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

	p.node, err = p.client.CoreV1().Nodes().Patch(context.TODO(),
		p.node.Name, types.JSONPatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
}
