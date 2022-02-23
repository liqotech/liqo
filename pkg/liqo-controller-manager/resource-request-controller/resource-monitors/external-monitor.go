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

package resourcemonitors

import (
	"context"
	"errors"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

// ExternalResourceMonitor is an object that keeps track of the cluster's resources.
type ExternalResourceMonitor struct {
	ResourceReaderClient
}

// NewExternalMonitor creates a new ExternalResourceMonitor.
func NewExternalMonitor(address string) (*ExternalResourceMonitor, error) {
	klog.Infof("Connecting to %s", address)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	conn, err := grpc.DialContext(ctx, address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	cancel()
	if err != nil {
		klog.Errorf("Could not connect to external resource monitor at %s: %s", address, err)
		return nil, err
	}
	client := NewResourceReaderClient(conn)
	return &ExternalResourceMonitor{
		ResourceReaderClient: client,
	}, nil
}

// Register sets an update notifier.
func (m *ExternalResourceMonitor) Register(ctx context.Context, notifier ResourceUpdateNotifier) {
	stream, err := m.ResourceReaderClient.Subscribe(ctx, &SubscribeRequest{})
	if err != nil {
		klog.Errorf("grpc error while subscribing: %s", err)
	}
	go func() {
		for {
			_, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				klog.V(4).Infof("The external monitor closed the Register() stream")
				break
			}
			if err != nil {
				klog.Errorf("grpc error while receiving notifications: %s", err)
				continue
			}
			notifier.NotifyChange()
		}
	}()
}

// ReadResources reads the resources from the upstream API.
func (m *ExternalResourceMonitor) ReadResources(clusterID string) corev1.ResourceList {
	response, err := m.ResourceReaderClient.ReadResources(context.Background(), &ReadRequest{Originator: clusterID})
	if err != nil {
		klog.Errorf("grpc error: %s", err)
		return corev1.ResourceList{}
	}
	ret := corev1.ResourceList{}
	for key, value := range response.Resources {
		apiQty, err := resource.ParseQuantity(value)
		if err != nil {
			klog.Errorf("deserialization error: %s", err)
			continue
		}
		ret[corev1.ResourceName(key)] = apiQty
	}
	return ret
}

// RemoveClusterID calls the method on the upstream API.
func (m *ExternalResourceMonitor) RemoveClusterID(clusterID string) {
	_, err := m.ResourceReaderClient.RemoveCluster(context.Background(), &RemoveRequest{Cluster: clusterID})
	if err != nil {
		klog.Errorf("grpc error: %s", err)
	}
}
