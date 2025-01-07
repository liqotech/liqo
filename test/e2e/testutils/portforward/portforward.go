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

package portforward

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RunPortForwardContext implements all the necessary functionality for port-forward cmd.
// It ends portforwarding when an error is received from the backend, or an interrupt
// signal is received, or the provided context is done.
func RunPortForwardContext(ctx context.Context, cfg *rest.Config, cl client.Client, podName, podNamespace string, localPort, targetPort int) error {
	ppf := NewPodPortForwarderOptions(cfg, cl, podName, podNamespace, localPort, targetPort)

	// Managing termination signal from the terminal.
	// The stopCh gets closed to gracefully handle its termination.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	returnCtx, returnCtxCancel := context.WithCancel(ctx)
	defer returnCtxCancel()

	go func() {
		select {
		case <-signals:
		case <-returnCtx.Done():
		}
		if ppf.StopCh != nil {
			close(ppf.StopCh)
		}
	}()

	return ppf.PortForwardPod(ctx)
}

// NewPodPortForwarderOptions creates a new PodPortForwarderOptions.
func NewPodPortForwarderOptions(cfg *rest.Config, cl client.Client,
	podName, podNamespace string, localPort, targetPort int) *PodPortForwarderOptions {
	return &PodPortForwarderOptions{
		Config:       cfg,
		Client:       cl,
		PodName:      podName,
		PodNamespace: podNamespace,
		LocalPort:    localPort,
		TargetPort:   targetPort,
		Streams:      genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		StopCh:       make(chan struct{}, 1),
		ReadyCh:      make(chan struct{}),
	}
}

// PodPortForwarderOptions contains the options for the port forwarding a pod.
type PodPortForwarderOptions struct {
	// RestConfig is the kubernetes config
	Config *rest.Config
	// Client is the controller-runtime client
	Client client.Client
	// PodName is the name of the pod to port forward
	PodName string
	// PodNamespace is the namespace of the pod to port forward
	PodNamespace string
	// LocalPort is the local port that will be selected to expose the PodPort
	LocalPort int
	// TargetPort is the target port for the pod
	TargetPort int
	// Steams configures where to write or read input from
	Streams genericclioptions.IOStreams
	// StopCh is the channel used to manage the port forward lifecycle
	StopCh chan struct{}
	// ReadyCh communicates when the tunnel is ready to receive traffic
	ReadyCh chan struct{}
}

// PortForwardPod port forwards the pod.
func (o *PodPortForwarderOptions) PortForwardPod(ctx context.Context) error {
	// Check if pod is in Running phase
	var pod corev1.Pod
	if err := o.Client.Get(ctx, client.ObjectKey{Namespace: o.PodNamespace, Name: o.PodName}, &pod); err != nil {
		return err
	}
	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("unable to forward port because pod is not running. Current status=%v", pod.Status.Phase)
	}

	pwPath := fmt.Sprintf("api/v1/namespaces/%s/pods/%s/portforward", o.PodNamespace, o.PodName)
	pwURL, err := url.Parse(fmt.Sprintf("%s/%s", o.Config.Host, pwPath))
	if err != nil {
		return err
	}

	transport, upgrader, err := spdy.RoundTripperFor(o.Config)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, pwURL)

	fw, err := portforward.New(dialer,
		[]string{fmt.Sprintf("%d:%d", o.LocalPort, o.TargetPort)},
		o.StopCh, o.ReadyCh,
		o.Streams.Out, o.Streams.ErrOut)
	if err != nil {
		return err
	}

	return fw.ForwardPorts()
}
