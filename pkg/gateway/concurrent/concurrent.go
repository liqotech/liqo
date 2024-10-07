// Copyright 2019-2024 The Liqo Authors
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

package concurrent

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ manager.Runnable = &Runnable{}

// Runnable is a Runnable that manages concurrency.
type Runnable struct {
	Client client.Client

	PodName        string
	DeploymentName string
	Namespace      string
}

// NewRunnable creates a new Runnable.
func NewRunnable(cl client.Client, podName, deploymentName, namespace string) *Runnable {
	return &Runnable{
		Client:         cl,
		PodName:        podName,
		DeploymentName: deploymentName,
		Namespace:      namespace,
	}
}

// Start starts the ConcurrentRunnable.
func (cr *Runnable) Start(ctx context.Context) error {
	pods, err := ListAllGatewaysReplicas(ctx, cr.Client, cr.Namespace, cr.DeploymentName)
	if err != nil {
		return err
	}

	for i := range pods {
		if pods[i].GetName() == cr.PodName {
			fmt.Printf("Adding active gateway label to pod %s\n", client.ObjectKeyFromObject(&pods[i]))
			if err := AddActiveGatewayLabel(ctx, cr.Client, client.ObjectKeyFromObject(&pods[i])); err != nil {
				return err
			}
		} else {
			fmt.Printf("Removing active gateway label from pod %s\n", client.ObjectKeyFromObject(&pods[i]))
			if err := RemoveActiveGatewayLabel(ctx, cr.Client, client.ObjectKeyFromObject(&pods[i])); err != nil {
				return err
			}
		}
	}
	fmt.Println("Runnable ended")
	return nil
}
