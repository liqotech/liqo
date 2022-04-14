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
	"fmt"
	"net"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

type FakeGRPCServer struct {
	Ready sync.WaitGroup // Gets unlocked when the server is ready

	Server *grpc.Server
	ResourceReaderServer
}

func (b *FakeGRPCServer) Start(ctx context.Context) error {
	address := fmt.Sprintf("%s%d", "127.0.0.1:", 7000)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	klog.Infof("Listening on %s", address)
	b.Server = grpc.NewServer()
	RegisterResourceReaderServer(b.Server, b)
	go func() {
		<-ctx.Done()
		klog.Infof("Stopping gracefully")
		b.Server.GracefulStop()
	}()
	b.Ready.Done()
	return b.Server.Serve(lis)
}

func (b *FakeGRPCServer) ReadResources(context.Context, *ReadRequest) (*ReadResponse, error) {
	resources := corev1.ResourceList{}
	resources[corev1.ResourceCPU] = resource.MustParse("1000")
	resources[corev1.ResourceMemory] = resource.MustParse("200e6")
	protobufResponse := &ReadResponse{Resources: map[string]string{}}
	for name, value := range resources {
		protobufResponse.Resources[name.String()] = value.String()
	}
	return protobufResponse, nil
}

// Subscribe pushes one update then closes the subscription.
func (b *FakeGRPCServer) Subscribe(req *SubscribeRequest, srv ResourceReader_SubscribeServer) error {
	return srv.Send(&UpdateNotification{})
}

func (b *FakeGRPCServer) RemoveCluster(context.Context, *RemoveRequest) (*RemoveResponse, error) {
	return &RemoveResponse{}, nil
}

const timeout = 1

var fakeServer = FakeGRPCServer{}
var grpcCtx, grpcCancel = context.WithCancel(context.Background())

var _ = BeforeSuite(func() {
	fakeServer.Ready.Add(1)
	go func() {
		defer GinkgoRecover()
		Expect(fakeServer.Start(grpcCtx)).To(Succeed())
	}()
})
var _ = AfterSuite(func() {
	grpcCancel()
})

var _ = Describe("ResourceMonitors Suite", func() {
	Context("ExternalMonitor", func() {
		var monitor *ExternalResourceMonitor

		It("Connects", func() {
			fakeServer.Ready.Wait()
			extMonitor, err := NewExternalMonitor(grpcCtx, "127.0.0.1:7000")
			Expect(err).ToNot(HaveOccurred())
			monitor = extMonitor
		}, timeout)
		It("Reads resources", func() {
			fakeServer.Ready.Wait()
			resources := monitor.ReadResources(context.Background(), "")
			Expect(resources.Cpu().Equal(resource.MustParse("1000"))).To(BeTrue())
			Expect(resources.Memory().Equal(resource.MustParse("200e6"))).To(BeTrue())
		}, timeout)
		It("Receives update notifications", func() {
			fakeServer.Ready.Wait()
			timeoutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			subscription, err := monitor.Subscribe(timeoutCtx, &SubscribeRequest{})
			Expect(err).ToNot(HaveOccurred())
			_, err = subscription.Recv()
			Expect(err).ToNot(HaveOccurred())
		}, timeout)
	})
})
