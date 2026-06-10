// Copyright 2019-2026 The Liqo Authors
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

package dra

import (
	"context"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	resourcev1clients "k8s.io/client-go/kubernetes/typed/resource/v1"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// ensureRemoteDeviceClass copies a DeviceClass from local to remote when missing on the
// remote cluster. It returns nil if the class already exists or was successfully created;
// it returns an error if the local class is missing or if the remote create failed for
// reasons other than AlreadyExists.
//
// Per design, reflected DeviceClasses are never garbage-collected: once present on the
// remote, they remain.
func ensureRemoteDeviceClass(
	ctx context.Context,
	name string,
	localClient, remoteClient resourcev1clients.DeviceClassInterface,
	labelsNotReflected, annotationsNotReflected []string,
) error {
	_, err := remoteClient.Get(ctx, name, metav1.GetOptions{})
	switch {
	case err == nil:
		return nil
	case !kerrors.IsNotFound(err):
		return fmt.Errorf("getting remote DeviceClass %q: %w", name, err)
	}

	local, err := localClient.Get(ctx, name, metav1.GetOptions{})
	switch {
	case kerrors.IsNotFound(err):
		return fmt.Errorf("cannot reflect DeviceClass %q that does not exist locally: %w", name, err)
	case err != nil:
		return fmt.Errorf("getting local DeviceClass %q: %w", name, err)
	}

	remote := forge.RemoteDeviceClass(local, labelsNotReflected, annotationsNotReflected)
	if _, err := remoteClient.Create(ctx, remote, metav1.CreateOptions{}); err != nil && !kerrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating remote DeviceClass %q: %w", name, err)
	}
	klog.Infof("Reflected DeviceClass %q to remote cluster", name)
	return nil
}
