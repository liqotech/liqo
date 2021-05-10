package namespace_controller

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/slice"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// Checks if Namespace has all offloading Labels of a specific node
func checkOffloadingLabels(na *corev1.Namespace, n *corev1.Node) bool {
	for key := range n.GetLabels() {
		if strings.HasPrefix(key, offloadingPrefixLabel) {
			if _, ok := na.GetLabels()[key]; !ok {
				klog.Infof(" Namespace '%s' cannot be offloaded on remote cluster: %s", na.GetName(), n.Annotations[liqoconst.VirtualNodeClusterId])
				return false
			}
		}
	}
	return true
}

// Checks if mappingLabel value is changed from the previous one
func mappingLabelUpdate(oldLabels map[string]string, newLabels map[string]string) bool {
	ret := false
	if val1, ok := oldLabels[mappingLabel]; ok {
		ret = val1 != newLabels[mappingLabel]
	}
	return ret
}

// Checks if the Namespace which triggers an Event, contains mappingLabel
func mappingLabelPresence(labels map[string]string) bool {
	_, ok := labels[mappingLabel]
	return ok
}

// Events not filtered:
// 1 -- deletion timestamp is updated on a relevant namespace (only that ones with my finalizer)
// 2 -- add/delete labels, and mappingLabel is present before or after the update
// 3 -- update the value of mappingLabel label only
// 4 -- add namespace with at least mappingLabel
func manageLabelPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// if a namespace with namespaceControllerFinalizer is deleted, trigger Reconcile
			if !(e.MetaNew.GetDeletionTimestamp().IsZero()) && slice.ContainsString(e.MetaNew.GetFinalizers(), namespaceControllerFinalizer, nil) {
				return true
			}

			// if the number of labels is changed after the event, and before or after the event there was mappingLabel, maybe controller has to do something, so trigger it
			// ||
			// if mappingLabel value is changed while the namespace is offloaded, controller has to force mappingLabel to its old value (see addDesiredMapping function)
			return ((len(e.MetaOld.GetLabels()) != len(e.MetaNew.GetLabels())) && (mappingLabelPresence(e.MetaOld.GetLabels()) ||
				mappingLabelPresence(e.MetaNew.GetLabels()))) || mappingLabelUpdate(e.MetaOld.GetLabels(), e.MetaNew.GetLabels())
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return mappingLabelPresence(e.Meta.GetLabels())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
}
