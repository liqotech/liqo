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
		return err
	}

	fmt.Println("* Initializing... 🔌 ")
	k8sClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}

	if t.ResticPassword == "" {
		t.ResticPassword = utils.RandomString(16)
	}

	fmt.Println("* Processing Volume Moving... 💾 ")
	return processMoveVolume(ctx, t, k8sClient)
}

func processMoveVolume(ctx context.Context, t *Args, k8sClient client.Client) error {
	// TODO
	return nil
}
