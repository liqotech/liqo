// Copyright 2019-2022 The Liqo Authors
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

package netns

import (
	"runtime"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/gruntwork-io/gruntwork-cli/errors"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

const (
	nsPath = "/run/netns/"
)

// CreateNetns given a name it will check if a namespace exists with the given name
// and will remove it. Then the namespace will be recreated. To start fresh with a clean
// network namespace is preferred since we create a veth pair between network namespaces.
// If the namespace exists it means that our operator has crashed, better clean the namespace,
// because it's hard to check the existing configuration that spans multiple network namespaces.
// Returns a handler to the newly created network namespace or an error in case
// something goes wrong.
func CreateNetns(name string) (ns.NetNS, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Get current namespace.
	currentNs, err := ns.GetCurrentNS()
	if err != nil {
		return nil, err
	}
	namespacePath := nsPath + name
	err = DeleteNetns(name)
	if err != nil {
		return nil, err
	}
	// Create a new network namespace.
	_, err = netns.NewNamed(name)
	if err != nil {
		return nil, err
	}
	netNamespace, err := ns.GetNS(namespacePath)
	if err != nil {
		return nil, err
	}
	// Set back the original namespace.
	if err = currentNs.Set(); err != nil {
		return nil, err
	}
	return netNamespace, nil
}

// DeleteNetns removes a given network namespace by name.
// If the namespace does not exist does nothing, in case of error returns it.
func DeleteNetns(name string) error {
	if err := netns.DeleteNamed(name); err != nil && !errors.IsError(err, unix.ENOENT) {
		klog.Errorf("an error occurred while removing network namespace with name %s: %v", name, err)
		return err
	}
	return nil
}
