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

package concurrent

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/liqotech/liqo/pkg/utils/ipc"
)

var _ manager.Runnable = &RunnableGateway{}

// RunnableGateway is a RunnableGateway that manages concurrency.
type RunnableGateway struct {
	Client client.Client

	PodName     string
	GatewayName string
	Namespace   string

	Socket           net.Listener
	GuestConnections ipc.GuestConnections
}

// NewRunnableGatewayStartup creates a new Runnable.
func NewRunnableGatewayStartup(cl client.Client, podName, gatewayName, namespace string, containerNames []string) (*RunnableGateway, error) {
	guestConnections := ipc.NewGuestConnections(containerNames)

	socket, err := ipc.CreateListenSocket(unixSocketPath)
	if err != nil {
		return nil, err
	}

	err = ipc.WaitAllGuestsConnections(guestConnections, socket)
	if err != nil {
		return nil, err
	}

	return &RunnableGateway{
		Client:           cl,
		PodName:          podName,
		GatewayName:      gatewayName,
		Namespace:        namespace,
		Socket:           socket,
		GuestConnections: guestConnections,
	}, nil
}

// Start starts the ConcurrentRunnable.
func (rg *RunnableGateway) Start(ctx context.Context) error {
	defer rg.Close()

	pods, err := ListAllGatewaysReplicas(ctx, rg.Client, rg.Namespace, rg.GatewayName)
	if err != nil {
		return err
	}

	var activePod *corev1.Pod

	for i := range pods {
		if pods[i].GetName() == rg.PodName {
			activePod = &pods[i]
		} else {
			if err := RemoveActiveGatewayLabel(ctx, rg.Client, client.ObjectKeyFromObject(&pods[i])); err != nil {
				return err
			}
		}
	}

	if activePod == nil {
		return fmt.Errorf("active gateway pod not found")
	}

	if err := AddActiveGatewayLabel(ctx, rg.Client, client.ObjectKeyFromObject(activePod)); err != nil {
		return err
	}

	if err := ipc.StartAllGuestsConnections(rg.GuestConnections); err != nil {
		return err
	}

	return nil
}

// Close closes the Runnable.
func (rg *RunnableGateway) Close() {
	ipc.CloseListenSocket(rg.Socket)
	ipc.CloseAllGuestsConnections(rg.GuestConnections)
}
