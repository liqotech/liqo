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

package resourcemonitors

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
)

var _ ResourceReader = &ExternalResourceMonitor{}

// ExternalResourceMonitor is an object that keeps track of the cluster's resources.
type ExternalResourceMonitor struct {
	ResourceReaderClient
}

// NewExternalMonitor creates a new ExternalResourceMonitor.
func NewExternalMonitor(ctx context.Context, address string, connectionTimeout time.Duration) (*ExternalResourceMonitor, error) {
	klog.Infof("Connecting to %s", address)
	ctx, cancel := context.WithTimeout(ctx, connectionTimeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to grpc server %s: %w", address, err)
	}
	client := NewResourceReaderClient(conn)
	return &ExternalResourceMonitor{
		ResourceReaderClient: client,
	}, nil
}

// Register sets an update notifier.
func (m *ExternalResourceMonitor) Register(ctx context.Context, notifier ResourceUpdateNotifier) {
	var err error
	var stream ResourceReader_SubscribeClient
	go func() {
		for ctx.Err() == nil {
			if stream == nil {
				if stream, err = m.Subscribe(ctx, &Empty{}); err != nil {
					klog.Errorf("Failed to subscribe to the external resource monitor: %s", err)
					// Prevent busy loop in case of continuous errors
					timeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
					<-timeout.Done()
					cancel()
					continue
				}
				// A unidirectional stream is enough to get notified by the server
				err := stream.CloseSend()
				if err != nil {
					klog.Warningf("error while making stream unidirectional to external resource monitor: %v", err)
				}
			}
			notification, err := stream.Recv()
			if err != nil {
				klog.Errorf("stream to external resource monitor server closed due to an error: %v", err)

				// this will force the retry
				stream = nil
				continue
			}
			notifier.NotifyChange(notification.ClusterID)
		}
	}()
}

// ReadResources reads the resources from the upstream API.
func (m *ExternalResourceMonitor) ReadResources(ctx context.Context, clusterID string) ([]*ResourceList, error) {
	response, err := m.ResourceReaderClient.ReadResources(ctx, &ClusterIdentity{ClusterID: clusterID})
	if err != nil {
		return nil, err
	}
	/*for i, resourceList := range response.ResourceLists {
		for key, value := range resourceList.Resources {
			if ret[i] == nil {
				ret[i] = make(corev1.ResourceList)
			}
			if value != nil {
				ret[i][corev1.ResourceName(key)] = *value
			} else {
				ret[i][corev1.ResourceName(key)] = resource.MustParse("0")
			}
		}
	}*/
	return response.ResourceLists, nil
}

// RemoveClusterID calls the method on the upstream API.
func (m *ExternalResourceMonitor) RemoveClusterID(ctx context.Context, clusterID string) error {
	_, err := m.ResourceReaderClient.RemoveCluster(ctx, &ClusterIdentity{ClusterID: clusterID})
	if err != nil {
		return err
	}
	return nil
}
