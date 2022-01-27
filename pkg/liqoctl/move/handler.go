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

package move

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
	"github.com/liqotech/liqo/pkg/utils"
)

// Args encapsulates arguments required to move a resource.
type Args struct {
	VolumeName string
	Namespace  string
	TargetNode string

	ResticPassword string
}

// HandleMoveVolumeCommand handles the move volume command,
// configuring all the resources required to move a liqo volume.
func HandleMoveVolumeCommand(ctx context.Context, t *Args) error {
	restConfig, err := common.GetLiqoctlRestConf()
	if err != nil {
		common.ErrorPrinter.Printf("Error while getting rest config: %v\n", err)
		return err
	}

	printer := common.NewPrinter("", common.Cluster1Color)

	s, err := printer.Spinner.Start("Initializing")
	utilruntime.Must(err)

	k8sClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		s.Fail("Failed to create k8s client %v", err)
		return err
	}
	s.Success("Client initialized")

	if t.ResticPassword == "" {
		t.ResticPassword = utils.RandomString(16)
	}

	return processMoveVolume(ctx, t, k8sClient, printer)
}

func processMoveVolume(ctx context.Context, t *Args, k8sClient client.Client, printer *common.Printer) error {
	// we need a context that is not canceled even if the user press Ctrl+C
	deferCtx := context.Background()

	s, err := printer.Spinner.Start("Running pre-flight checks")
	utilruntime.Must(err)

	var pvc corev1.PersistentVolumeClaim
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: t.Namespace, Name: t.VolumeName}, &pvc); err != nil {
		s.Fail(fmt.Sprintf("Failed to get PVC %s/%s: %v", t.Namespace, t.VolumeName, err))
		return err
	}

	err = checkNoMounter(ctx, k8sClient, &pvc)
	if err != nil {
		s.Fail("Failed to check mounter pod: ", err)
		return err
	}
	s.Success("Pre-flight checks passed")

	s, err = printer.Spinner.Start("Offloading the liqo-storage namespace")
	utilruntime.Must(err)

	var targetNode corev1.Node
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: t.TargetNode}, &targetNode); err != nil {
		s.Fail("Failed to get target node: ", err)
		return err
	}
	targetIsLocal := !utils.IsVirtualNode(&targetNode)

	originIsLocal, originNode, err := isLocalVolume(ctx, k8sClient, &pvc)
	if err != nil {
		s.Fail("Failed to check if the volume is local: ", err)
		return err
	}

	if err = offloadLiqoStorageNamespace(ctx, k8sClient, originNode, &targetNode); err != nil {
		s.Fail("Failed to offload the liqo-storage namespace: ", err)
		return err
	}
	s.Success("Liqo-storage namespace offloaded")

	defer func() {
		s, err = printer.Spinner.Start("Repatriating the liqo-storage namespace")
		utilruntime.Must(err)

		if err := repatriateLiqoStorageNamespace(deferCtx, k8sClient); err != nil {
			s.Fail("Failed to repatriate the liqo-storage namespace: ", err)
			return
		}
		s.Success("Repatriated the liqo-storage namespace")
	}()

	s, err = printer.Spinner.Start("Ensuring restic repository")
	utilruntime.Must(err)

	if err := ensureResticRepository(ctx, k8sClient, &pvc); err != nil {
		s.Fail("Failed to ensure restic repository: ", err)
		return err
	}
	s.Success("Ensured restic repository")

	defer func() {
		s, err = printer.Spinner.Start("Removing restic repository")
		utilruntime.Must(err)

		if err = deleteResticRepository(deferCtx, k8sClient); err != nil {
			s.Fail("Failed to remove restic repository: ", err)
			return
		}
		s.Success("Removed restic repository")
	}()

	s, err = printer.Spinner.Start("Waiting for restic repository to be up and running")
	utilruntime.Must(err)

	if err = waitForResticRepository(ctx, k8sClient); err != nil {
		s.Fail("Failed to wait for restic repository to be up and running: ", ctx.Err())
		return err
	}
	s.Success("Restic repository is up and running")

	s, err = printer.Spinner.Start("Taking snapshot")
	utilruntime.Must(err)

	originResticRepositoryURL, err := getResticRepositoryURL(ctx, k8sClient, originIsLocal)
	if err != nil {
		s.Fail("Failed to get origin restic repository URL: ", err)
		return err
	}
	if err = takeSnapshot(ctx, k8sClient, &pvc,
		originResticRepositoryURL, t.ResticPassword); err != nil {
		s.Fail("Failed to take snapshot: ", err)
		return err
	}
	s.Success("Snapshot taken")

	s, err = printer.Spinner.Start("Moving the volume")
	utilruntime.Must(err)

	newPvc, err := recreatePvc(ctx, k8sClient, &pvc)
	if err != nil {
		s.Fail("Failed to recreate PVC: ", err)
		return err
	}

	targetResticRepositoryURL, err := getResticRepositoryURL(ctx, k8sClient, targetIsLocal)
	if err != nil {
		s.Fail("Failed to get target restic repository URL: ", err)
		return err
	}
	if err = restoreSnapshot(ctx, k8sClient,
		&pvc, newPvc, t.TargetNode,
		targetResticRepositoryURL, t.ResticPassword); err != nil {
		s.Fail("Failed to restore snapshot: ", err)
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

	return fmt.Sprintf("rest:http://%s.%s.svc.cluster.local:%d/", resticRegistry, namespace, resticPort), nil
}
