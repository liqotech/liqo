// Copyright 2019-2021 The Liqo Authors
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
	"time"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/gruntwork-io/gruntwork-cli/errors"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	liqoneterrors "github.com/liqotech/liqo/pkg/liqonet/errors"
	liqoutils "github.com/liqotech/liqo/pkg/liqonet/utils"
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

// CreateVethPair it will create veth pair in originNetns and move one of them in dstNetns.
// originNetns is the host netns and dstNetns is the gateway netns.
// Error is returned if something goes wrong.
func CreateVethPair(originVethName, dstVethName string, originNetns, dstNetns ns.NetNS, linkMTU int) error {
	if originNetns == nil || dstNetns == nil {
		return &liqoneterrors.WrongParameter{
			Parameter: "originNetns and dstNetns",
			Reason:    liqoneterrors.NotNil}
	}
	// Check if in originNetns, aka host netns, exists an interface named as originVethName.
	// If it exists than we remove it.
	err := originNetns.Do(func(currentNetns ns.NetNS) error {
		return liqoutils.DeleteIFaceByName(originVethName)
	})
	if err != nil {
		klog.Errorf("an error occurred while deleting interface {%s} in host network: %v", originVethName, err)
		return err
	}
	var createVethPair = func(hostNS ns.NetNS) error {
		_, _, err := ip.SetupVethWithName(originVethName, dstVethName, linkMTU, dstNetns)
		if err != nil {
			klog.Errorf("an error occurred while creating veth pair between host and gateway namespace: %v", err)
			return err
		}
		return nil
	}
	// If we just delete the old network namespace it would require some time for the kernel to
	// remove the veth device in the host network, so we retry in case of temporary conflicts.
	retryiable := func(err error) bool {
		return true
	}
	tryToCreateVeth := func() error {
		return originNetns.Do(createVethPair)
	}
	return retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 100 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}, retryiable, tryToCreateVeth)
}
