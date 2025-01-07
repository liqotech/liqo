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

package nsoffctrl

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

// Removes right entry from one NamespaceMap, if present.
func removeDesiredMapping(ctx context.Context, c client.Client, localName string, nm *offloadingv1beta1.NamespaceMap) error {
	if nm.Spec.DesiredMapping == nil {
		klog.V(4).Infof("NamespaceMap %q does not contain any entry", nm.GetName())
		return nil
	}
	if _, ok := nm.Spec.DesiredMapping[localName]; ok {
		original := nm.DeepCopy()
		delete(nm.Spec.DesiredMapping, localName)
		if err := c.Patch(ctx, nm, client.MergeFrom(original)); err != nil {
			klog.Errorf("Unable to remove entry for namespace %q from NamespaceMap %q: %v", localName, nm.GetName(), err)
			return err
		}
		klog.Infof("Entry for namespace %q correctly deleted from the NamespaceMap %q", localName, nm.GetName())
	}
	return nil
}

// Removes right entries from more than one NamespaceMap (it depends on len(nms)).
func removeDesiredMappings(ctx context.Context, c client.Client, localName string, nms map[string]*offloadingv1beta1.NamespaceMap) error {
	errorCondition := false
	for _, nm := range nms {
		if err := removeDesiredMapping(ctx, c, localName, nm); err != nil {
			errorCondition = true
		}
	}

	if errorCondition {
		return fmt.Errorf("some desiredMappings have not been deleted")
	}
	return nil
}

// Adds right entry on one NamespaceMap, if it isn't already there.
func addDesiredMapping(ctx context.Context, c client.Client, localName, remoteName string, nm *offloadingv1beta1.NamespaceMap) error {
	if nm.Spec.DesiredMapping == nil {
		nm.Spec.DesiredMapping = map[string]string{}
	}

	if current, ok := nm.Spec.DesiredMapping[localName]; !ok || current != remoteName {
		original := nm.DeepCopy()
		nm.Spec.DesiredMapping[localName] = remoteName
		if err := c.Patch(ctx, nm, client.MergeFrom(original)); err != nil {
			klog.Errorf("Unable to add entry for namespace %q to NamespaceMap %q: %v", localName, nm.GetName(), err)
			return err
		}

		klog.Infof("Entry for namespace %q successfully added to NamespaceMap %q", localName, nm.GetName())
	}
	return nil
}
