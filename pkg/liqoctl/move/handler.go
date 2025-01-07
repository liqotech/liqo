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

package move

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/pod"
)

// Options encapsulates the arguments of the move volume command.
type Options struct {
	*factory.Factory

	VolumeName string
	TargetNode string

	ContainersCPURequests, ContainersCPULimits resource.Quantity
	ContainersRAMRequests, ContainersRAMLimits resource.Quantity

	ResticPassword string

	ResticServerImage string
	ResticImage       string
}

// Run implements the move volume command.
func (o *Options) Run(ctx context.Context) error {
	// we need a context that is not canceled even if the user press Ctrl+C
	deferCtx := context.Background()

	s := o.Printer.StartSpinner("Running pre-flight checks")

	var pvc corev1.PersistentVolumeClaim
	if err := o.CRClient.Get(ctx, client.ObjectKey{Namespace: o.Namespace, Name: o.VolumeName}, &pvc); err != nil {
		s.Fail(fmt.Sprintf("Failed to get PVC %s/%s: %v", o.Namespace, o.VolumeName, output.PrettyErr(err)))
		return err
	}

	err := checkNoMounter(ctx, o.CRClient, &pvc)
	if err != nil {
		s.Fail("Failed to check mounter pod: ", output.PrettyErr(err))
		return err
	}
	s.Success("Pre-flight checks passed")

	s = o.Printer.StartSpinner("Offloading the liqo-storage namespace")

	var targetNode corev1.Node
	if err := o.CRClient.Get(ctx, client.ObjectKey{Name: o.TargetNode}, &targetNode); err != nil {
		s.Fail("Failed to get target node: ", output.PrettyErr(err))
		return err
	}
	targetIsLocal := !utils.IsVirtualNode(&targetNode)

	originIsLocal, originNode, err := isLocalVolume(ctx, o.CRClient, &pvc)
	if err != nil {
		s.Fail("Failed to check if the volume is local: ", output.PrettyErr(err))
		return err
	}

	if err = offloadLiqoStorageNamespace(ctx, o.CRClient, originNode, &targetNode); err != nil {
		s.Fail("Failed to offload the liqo-storage namespace: ", output.PrettyErr(err))
		return err
	}
	s.Success("Liqo-storage namespace offloaded")

	defer func() {
		s = o.Printer.StartSpinner("Repatriating the liqo-storage namespace")

		if err := repatriateLiqoStorageNamespace(deferCtx, o.CRClient); err != nil {
			s.Fail("Failed to repatriate the liqo-storage namespace: ", output.PrettyErr(err))
			return
		}
		s.Success("Repatriated the liqo-storage namespace")
	}()

	s = o.Printer.StartSpinner("Ensuring restic repository")

	if err := o.ensureResticRepository(ctx, &pvc); err != nil {
		s.Fail("Failed to ensure restic repository: ", output.PrettyErr(err))
		return err
	}
	s.Success("Ensured restic repository")

	defer func() {
		s = o.Printer.StartSpinner("Removing restic repository")

		if err = deleteResticRepository(deferCtx, o.CRClient); err != nil {
			s.Fail("Failed to remove restic repository: ", output.PrettyErr(err))
			return
		}
		s.Success("Removed restic repository")
	}()

	s = o.Printer.StartSpinner("Waiting for restic repository to be up and running")

	if err = waitForResticRepository(ctx, o.CRClient); err != nil {
		s.Fail("Failed to wait for restic repository to be up and running: ", output.PrettyErr(err))
		return err
	}
	s.Success("Restic repository is up and running")

	s = o.Printer.StartSpinner("Taking snapshot")

	originResticRepositoryURL, err := getResticRepositoryURL(ctx, o.CRClient, originIsLocal)
	if err != nil {
		s.Fail("Failed to get origin restic repository URL: ", output.PrettyErr(err))
		return err
	}
	if err = o.takeSnapshot(ctx, &pvc, originResticRepositoryURL); err != nil {
		s.Fail("Failed to take snapshot: ", output.PrettyErr(err))
		return err
	}
	s.Success("Snapshot taken")

	s = o.Printer.StartSpinner("Moving the volume")

	newPvc, err := recreatePvc(ctx, o.CRClient, &pvc)
	if err != nil {
		s.Fail("Failed to recreate PVC: ", output.PrettyErr(err))
		return err
	}

	targetResticRepositoryURL, err := getResticRepositoryURL(ctx, o.CRClient, targetIsLocal)
	if err != nil {
		s.Fail("Failed to get target restic repository URL: ", output.PrettyErr(err))
		return err
	}
	if err = o.restoreSnapshot(ctx, &pvc, newPvc, targetResticRepositoryURL); err != nil {
		s.Fail("Failed to restore snapshot: ", output.PrettyErr(err))
		return err
	}

	s.Success("Restore completed")
	return nil
}

func getResticRepositoryURL(ctx context.Context, cl client.Client, isLocal bool) (string, error) {
	var namespace string
	if isLocal {
		namespace = liqoStorageNamespace
	} else {
		var err error
		namespace, err = getRemoteStorageNamespaceName(ctx, cl, nil)
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("rest:http://%s.%s.svc:%d/", resticRegistry, namespace, resticPort), nil
}

func (o *Options) forgeContainerResources() corev1.ResourceRequirements {
	return pod.ForgeContainerResources(o.ContainersCPURequests, o.ContainersCPULimits, o.ContainersRAMRequests, o.ContainersRAMLimits)
}
