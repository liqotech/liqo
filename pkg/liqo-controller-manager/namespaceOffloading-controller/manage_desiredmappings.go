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

package namespaceoffloadingctrl

import (
	"context"
	"fmt"

	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualKubelet/v1alpha1"
)

// Removes right entry from one NamespaceMap, if present.
func removeDesiredMapping(c client.Client, localName string, nm *mapsv1alpha1.NamespaceMap) error {
	if _, ok := nm.Spec.DesiredMapping[localName]; ok {
		patch := nm.DeepCopy()
		delete(nm.Spec.DesiredMapping, localName)
		if err := c.Patch(context.TODO(), nm, client.MergeFrom(patch)); err != nil {
			klog.Errorf("%s --> Unable to patch NamespaceMap '%s'", err, nm.GetName())
			return err
		}
		klog.Infof(" Entry for the namespace '%s' is correctly deleted from the NamespaceMap '%s'", nm.GetName(), nm.GetName())
	}
	return nil
}

// Removes right entries from more than one NamespaceMap (it depends on len(nms)).
func removeDesiredMappings(c client.Client, localName string, nms map[string]*mapsv1alpha1.NamespaceMap) error {
	errorCondition := false
	for _, nm := range nms {
		if err := removeDesiredMapping(c, localName, nm); err != nil {
			errorCondition = true
		}
	}
	if errorCondition {
		err := fmt.Errorf("some desiredMappings has not been deleted")
		klog.Error(err)
		return err
	}
	return nil
}

// Adds right entry on one NamespaceMap, if it isn't already there.
func addDesiredMapping(c client.Client, localName, remoteName string,
	nm *mapsv1alpha1.NamespaceMap) error {
	if nm.Spec.DesiredMapping == nil {
		nm.Spec.DesiredMapping = map[string]string{}
	}

	if _, ok := nm.Spec.DesiredMapping[localName]; !ok {
		patch := nm.DeepCopy()
		nm.Spec.DesiredMapping[localName] = remoteName
		if err := c.Patch(context.TODO(), nm, client.MergeFrom(patch)); err != nil {
			klog.Errorf("%s --> Unable to add entry for namespace '%s' on NamespaceMap '%s'",
				err, localName, nm.GetName())
			return err
		}
		klog.Infof("Entry for the namespace '%s' is successfully added on the NamespaceMap '%s' ", localName, nm.GetName())
	}
	return nil
}

// Adds right entries on more than one NamespaceMap (it depends on len(nms)).
func addDesiredMappings(c client.Client, localName, remoteName string,
	nms map[string]*mapsv1alpha1.NamespaceMap) error {
	errorCondition := false
	for _, nm := range nms {
		if err := addDesiredMapping(c, localName, remoteName, nm); err != nil {
			errorCondition = true
		}
	}
	if errorCondition {
		err := fmt.Errorf("some desiredMappings has not been added")
		klog.Error(err)
		return err
	}
	return nil
}
