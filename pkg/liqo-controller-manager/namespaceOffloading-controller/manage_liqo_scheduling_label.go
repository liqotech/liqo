package namespaceoffloadingctrl

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

func addLiqoSchedulingLabel(ctx context.Context, c client.Client, namespaceName string) error {
	namespace := &corev1.Namespace{}
	if err := c.Get(ctx, types.NamespacedName{Name: namespaceName}, namespace); err != nil {
		klog.Errorf("%s --> Unable to get the namespace '%s'", err, namespaceName)
		return err
	}

	if value, ok := namespace.Labels[liqoconst.SchedulingLiqoLabel]; !ok || value != liqoconst.SchedulingLiqoLabelValue {
		if namespace.Labels == nil {
			namespace.Labels = map[string]string{}
		}
		namespace.Labels[liqoconst.SchedulingLiqoLabel] = liqoconst.SchedulingLiqoLabelValue
		if err := c.Update(ctx, namespace); err != nil {
			klog.Errorf(" %s --> Unable to add liqo scheduling label to the namespace '%s'", err, namespace.GetName())
			return err
		}
		klog.Infof(" Liqo scheduling label successfully added to the namespace '%s'", namespace.GetName())
	}
	return nil
}

func removeLiqoSchedulingLabel(ctx context.Context, c client.Client, namespaceName string) error {
	namespace := &corev1.Namespace{}
	if err := c.Get(ctx, types.NamespacedName{Name: namespaceName}, namespace); err != nil {
		klog.Errorf("%s --> Unable to get the namespace '%s'", err, namespaceName)
		return err
	}

	if value, ok := namespace.Labels[liqoconst.SchedulingLiqoLabel]; ok && value == liqoconst.SchedulingLiqoLabelValue {
		delete(namespace.Labels, liqoconst.SchedulingLiqoLabel)
		if err := c.Update(ctx, namespace); err != nil {
			klog.Errorf(" %s --> Unable to remove Liqo scheduling label from the namespace '%s'", err, namespace.GetName())
			return err
		}
		klog.Infof(" Liqo scheduling label successfully removed from the namespace '%s'", namespace.GetName())
	}
	return nil
}
