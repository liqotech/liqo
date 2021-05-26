package netns

import (
	"time"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/gruntwork-io/gruntwork-cli/errors"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"github.com/liqotech/liqo/pkg/liqonet"
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
	namespacePath := nsPath + name
	err := DeleteNetns(name)
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
// Error is returned if something goes wrong.
func CreateVethPair(originVethName, dstVethName string, originNetns, dstNetns ns.NetNS, linkMTU int) error {
	if originNetns == nil || dstNetns == nil {
		return &liqonet.WrongParameter{
			Parameter: "originNetns and dstNetns",
			Reason:    liqonet.NotNil}
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
