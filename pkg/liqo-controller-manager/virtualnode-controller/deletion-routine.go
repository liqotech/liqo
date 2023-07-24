// Copyright 2019-2023 The Liqo Authors
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

// Package virtualnodectrl contains VirtualNode Controller logic and some functions for managing NamespaceMap lifecycle.
// There are also some tests for VirtualNode Controller
package virtualnodectrl

import (
	"context"
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

var (
	deletionRoutineRunning = false
)

type DeletionRoutine struct {
	vnr                  *VirtualNodeReconciler
	wq                   workqueue.DelayingInterface
	virtualNodesDeleting map[string]interface{}
}

func RunDeletionRoutine(r *VirtualNodeReconciler) (*DeletionRoutine, error) {
	if deletionRoutineRunning {
		return nil, fmt.Errorf("deletion routine already running")
	}
	deletionRoutineRunning = true
	dr := &DeletionRoutine{
		vnr:                  r,
		wq:                   workqueue.NewDelayingQueue(),
		virtualNodesDeleting: make(map[string]interface{}),
	}
	go dr.run()
	return dr, nil
}

func (dr *DeletionRoutine) run() {
	pterm.FgGreen.Print("Starting deletion routine")
	ctx := context.TODO()
	wait.Forever(func() {
		vni, _ := dr.wq.Get()
		vn := vni.(*virtualkubeletv1alpha1.VirtualNode)
		pterm.FgGreen.Printfln("Deleting virtual node: %s", vn.Name)
		node, err := getters.GetNodeFromVirtualNode(ctx, dr.vnr.Client, vn)
		if err != nil {
			klog.Errorf("error getting node from virtual node: %v", err)
			return
		}

		if err := client.IgnoreNotFound(cordonNode(ctx, dr.vnr.Client, node)); err != nil {
			klog.Errorf("error cordoning node: %v", err)
			return
		}

		if err := client.IgnoreNotFound(drainNode(ctx, dr.vnr.Client, vn)); err != nil {
			klog.Errorf("error draining node: %v", err)
			return
		}

		if err := client.IgnoreNotFound(dr.vnr.Client.Delete(ctx, node, &client.DeleteOptions{})); err != nil {
			klog.Errorf("error deleting node: %v", err)
			return
		}

		err = dr.vnr.removeVirtualNodeFinalizer(ctx, vn)
		if err != nil {
			klog.Errorf(" %s --> Unable to remove the finalizer to the virtual-node %s in namespace %s", err, vn.Name, vn.Namespace)
		}

		delete(dr.virtualNodesDeleting, vn.Name)

		return
	}, time.Second)
}

// DeleteVirtualNode adds a virtual node to the deletion queue.
func (dr *DeletionRoutine) DeleteVirtualNode(vn *virtualkubeletv1alpha1.VirtualNode) {
	pterm.FgBlue.Println("Deleting virtual node")
	if _, ok := dr.virtualNodesDeleting[vn.Name]; ok {
		return
	}
	dr.virtualNodesDeleting[vn.Name] = struct{}{}
	pterm.FgRed.Println("Adding virtual node to deletion queue")
	dr.wq.Add(vn)
}
