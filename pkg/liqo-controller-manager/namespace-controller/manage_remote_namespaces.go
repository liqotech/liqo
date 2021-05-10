/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
   http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package namespacectrl

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
)

// Removes right entry from one NamespaceMap
func (r *NamespaceReconciler) removeDesiredMapping(localName string, nm *mapsv1alpha1.NamespaceMap) error {
	if _, ok := nm.Spec.DesiredMapping[localName]; ok {
		delete(nm.Spec.DesiredMapping, localName)
		if err := r.Update(context.TODO(), nm); err != nil {
			klog.Errorf("%s --> Unable to update NamespaceMap '%s'", err, nm.GetName())
			return err
		}
		klog.Infof(" Entries deleted correctly from '%s'", nm.GetName())
	}

	return nil
}

// Removes right entries from more than one NamespaceMap (it depends on len(nms))
func (r *NamespaceReconciler) removeDesiredMappings(localName string, nms map[string]*mapsv1alpha1.NamespaceMap) error {
	for _, nm := range nms {
		if err := r.removeDesiredMapping(localName, nm); err != nil {
			return err
		}
	}
	return nil
}

// Adds right entry on one NamespaceMap, if it isn't already there
func (r *NamespaceReconciler) addDesiredMapping(n *corev1.Namespace, remoteName string,
	nm *mapsv1alpha1.NamespaceMap) error {
	if nm.Spec.DesiredMapping == nil {
		nm.Spec.DesiredMapping = map[string]string{}
	}

	if oldValue, ok := nm.Spec.DesiredMapping[n.GetName()]; ok {
		// if entries are already here, but mappingLabel has a different value from the previous one, we force
		// again the old value. Common user cannot change remote namespace name while the namespace is offloaded
		// onto remote clusters
		if oldValue != remoteName {
			n.GetLabels()[mappingLabel] = oldValue
			if err := r.Update(context.TODO(), n); err != nil {
				klog.Errorf("%s --> Unable to update '%s' label for namespace '%s' ",
					err, mappingLabel, nm.GetName())
				return err
			}
			klog.Infof("Label '%s' successfully updated on namespace '%s' ", mappingLabel, nm.GetName())
		}
	} else {
		nm.Spec.DesiredMapping[n.GetName()] = remoteName
		if err := r.Patch(context.TODO(), nm, client.Merge); err != nil {
			klog.Errorf("%s --> Unable to add entry for namespace '%s' on NamespaceMap '%s'",
				err, n.GetName(), nm.GetName())
			return err
		}
		klog.Infof("Entry for namespace '%s' successfully added on NamespaceMap '%s' ",
			n.GetName(), nm.GetName())
	}
	return nil
}

// Adds right entries on more than one NamespaceMap (it depends on len(nms)), if entries aren't already there
func (r *NamespaceReconciler) addDesiredMappings(n *corev1.Namespace, remoteName string,
	nms map[string]*mapsv1alpha1.NamespaceMap) error {
	for _, nm := range nms {
		if err := r.addDesiredMapping(n, remoteName, nm); err != nil {
			return err
		}
	}
	return nil
}
