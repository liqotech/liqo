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

package common

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	k8s "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

// PortForwardOptions contains all the options in order to port-forward a pod's port.
type PortForwardOptions struct {
	Namespace     string
	Selector      *metav1.LabelSelector
	Config        *restclient.Config
	Client        client.Client
	PortForwarder PortForwarder
	RemotePort    int
	LocalPort     int
	Ports         []string
	StopChannel   chan struct{}
	ReadyChannel  chan struct{}
}

// PortForwarder interface that a port forwarder needs to implement.
type PortForwarder interface {
	ForwardPorts(method string, podURL *url.URL, opts *PortForwardOptions) error
}

// DefaultPortForwarder default forwarder implementation used to forward ports.
type DefaultPortForwarder struct {
	genericclioptions.IOStreams
}

// ForwardPorts forwards the ports given in the options for the given pod url.
func (f *DefaultPortForwarder) ForwardPorts(method string, podURL *url.URL, opt *PortForwardOptions) error {
	errChan := make(chan error, 1)

	transport, upgrader, err := spdy.RoundTripperFor(opt.Config)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, podURL)

	pf, err := portforward.New(dialer, opt.Ports, opt.StopChannel, opt.ReadyChannel, f.Out, f.ErrOut)
	if err != nil {
		return fmt.Errorf("unable to port forward into pod %s: %w", podURL.String(), err)
	}

	go func() {
		errChan <- pf.ForwardPorts()
	}()

	select {
	case err = <-errChan:
		if err != nil {
			return fmt.Errorf("an error occurred while port forwarding into pod %s: %w", podURL.String(), err)
		}
	case <-opt.ReadyChannel:
		break
	}

	return nil
}

// RunPortForward starts the forwarding.
func (o *PortForwardOptions) RunPortForward(ctx context.Context) error {
	var err error
	// Get the local port used to forward the pod's port.
	o.LocalPort, err = getFreePort()
	if err != nil {
		return fmt.Errorf("unable to get a local port: %w", err)
	}

	podURL, err := o.getPodURL(ctx)
	if err != nil {
		return err
	}

	o.Ports = []string{fmt.Sprintf("%d:%d", o.LocalPort, o.RemotePort)}

	return o.PortForwarder.ForwardPorts(http.MethodPost, podURL, o)
}

// StopPortForward stops the forwarding.
func (o *PortForwardOptions) StopPortForward() {
	o.StopChannel <- struct{}{}
}

func (o *PortForwardOptions) getPodURL(ctx context.Context) (*url.URL, error) {
	lSelector, err := metav1.LabelSelectorAsSelector(&liqolabels.NetworkManagerPodLabelSelector)
	if err != nil {
		return nil, fmt.Errorf("an error occurred while retrieving network manager pod: %w", err)
	}

	fSelector := fields.OneTermEqualSelector("status.phase", string(v1.PodRunning))

	pod, err := getters.GetPodByLabel(ctx, o.Client, o.Namespace, lSelector, fSelector)
	if err != nil {
		return nil, fmt.Errorf("an error occurred while retrieving network manager pod: %w", err)
	}

	// Dirty trick used to build the URL.
	cl, err := k8s.NewForConfig(o.Config)
	if err != nil {
		return nil, fmt.Errorf("an error occurred while retrieving network manager pod: %w", err)
	}

	return cl.CoreV1().RESTClient().Post().Resource("pods").Namespace(pod.Namespace).Name(pod.Name).SubResource("portforward").URL(), nil
}
