package liqonodeprovider

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// cordonNode cordons the controlled node setting it in the unschedulable state.
func (p *LiqoNodeProvider) cordonNode(ctx context.Context) error {
	if err := p.patchNode(func(node *v1.Node) error {
		node.Spec.Unschedulable = true
		return nil
	}); err != nil {
		klog.Error(err)
		return err
	}

	return nil
}
