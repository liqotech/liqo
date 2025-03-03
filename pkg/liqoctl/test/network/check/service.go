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

package check

import (
	"context"
	"fmt"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/setup"
	"github.com/liqotech/liqo/pkg/liqoctl/test/utils"
)

var preferOrder = []corev1.NodeAddressType{
	corev1.NodeExternalDNS,
	corev1.NodeExternalIP,
	corev1.NodeInternalDNS,
	corev1.NodeInternalIP,
	corev1.NodeHostName,
}

// GetNodeAddress returns the address of the node.
func GetNodeAddress(node *corev1.Node) string {
	for _, addrType := range preferOrder {
		for _, addr := range node.Status.Addresses {
			if addr.Type == addrType {
				return addr.Address
			}
		}
	}
	return ""
}

// RunCheckPodToClusterIPService runs all the checks from the pod to the cluster IP service.
func RunCheckPodToClusterIPService(ctx context.Context, cl *client.Client, cfg client.Configs, opts *flags.Options,
	totreplicas int32) (successCount, errorCount int32, err error) {
	var successCountTot, errorCountTot int32

	svcName := fmt.Sprintf("%s.%s", setup.DeploymentName, setup.NamespaceName)

	for i := 0; i < int(totreplicas*2); i++ {
		successCount, errorCount, err = RunCheckToTargets(ctx, cl.Consumer, cfg[cl.ConsumerName],
			opts, cl.ConsumerName, []string{svcName}, false, ExecCurl)
		successCountTot += successCount
		errorCountTot += errorCount
		if err != nil {
			return successCountTot, errorCountTot, fmt.Errorf("consumer failed to run checks: %w", err)
		}
	}

	for k := range cl.Providers {
		// The test is repeated twice for each provider and consumer pods.
		// This is to ensure that all pods have been contacted from each other pods through the service.
		for i := 0; i < int(totreplicas*2); i++ {
			successCount, errorCount, err := RunCheckToTargets(ctx, cl.Providers[k], cfg[k],
				opts, k, []string{svcName}, false, ExecCurl)
			successCountTot += successCount
			errorCountTot += errorCount
			if err != nil {
				return successCountTot, errorCountTot, fmt.Errorf("provider %q failed to run checks: %w", k, err)
			}
		}
	}

	return successCountTot, errorCountTot, nil
}

// RunCheckExternalToNodePortServiceWithClient runs all the checks from the external to the node port service.
func RunCheckExternalToNodePortServiceWithClient(ctx context.Context, cl ctrlclient.Client,
	opts *flags.Options, totreplicas int32, httpclient *client.HTTPClient) (successCount, errorCount int32, err error) {
	svcnp := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: setup.DeploymentName + "np", Namespace: setup.NamespaceName},
	}

	if err := cl.Get(ctx, ctrlclient.ObjectKeyFromObject(&svcnp), &svcnp); err != nil {
		return 0, 0, fmt.Errorf("failed to get service: %w", err)
	}

	nodeport := svcnp.Spec.Ports[0].NodePort

	nodes := corev1.NodeList{}
	if err := cl.List(ctx, &nodes); err != nil {
		return 0, 0, fmt.Errorf("failed to list nodes: %w", err)
	}

	for i := 0; i < int(totreplicas*2); i++ {
		for i := range nodes.Items {
			if nodes.Items[i].GetLabels()[consts.TypeLabel] == consts.TypeNode {
				continue
			}
			if opts.NodePortNodes == flags.NodePortNodesWorkers && setup.IsNodeControlPlane(nodes.Items[i].Spec.Taints) {
				continue
			}
			if opts.NodePortNodes == flags.NodePortNodesControlPlanes && !setup.IsNodeControlPlane(nodes.Items[i].Spec.Taints) {
				continue
			}
			nodeip := GetNodeAddress(&nodes.Items[i])
			ok, err := httpclient.Curl(ctx, fmt.Sprintf("http://%s:%d", nodeip, nodeport), !opts.Topts.Verbose, opts.Topts.LocalFactory.Printer.Logger)
			successCount, errorCount, err = utils.ManageResults(opts.Topts.FailFast, err, ok, successCount, errorCount)
			if err != nil {
				return successCount, errorCount, err
			}
		}
	}
	return successCount, errorCount, nil
}

// RunsCheckExternalToNodePortService runs all the checks from the external to the node port service.
func RunsCheckExternalToNodePortService(ctx context.Context, cl *client.Client, opts *flags.Options,
	totreplicas int32) (successCountTot, errorCountTot int32, err error) {
	httpclient := client.NewHTTPClient(time.Second * 5)

	successCount, errorCount, err := RunCheckExternalToNodePortServiceWithClient(ctx, cl.Consumer, opts, totreplicas, httpclient)
	successCountTot += successCount
	errorCountTot += errorCount
	if err != nil {
		return successCount, errorCount, fmt.Errorf("consumer failed to run checks: %w", err)
	}
	for k := range cl.Providers {
		successCount, errorCount, err = RunCheckExternalToNodePortServiceWithClient(ctx, cl.Providers[k], opts, totreplicas, httpclient)
		successCountTot += successCount
		errorCountTot += errorCount
		if err != nil {
			return successCount, errorCount, fmt.Errorf("provider %q failed to run checks: %w", k, err)
		}
	}

	return successCountTot, errorCountTot, nil
}

// RunsCheckExternalToLoadBalancerServiceWithClient runs all the checks from the external to the load balancer service.
func RunsCheckExternalToLoadBalancerServiceWithClient(ctx context.Context, cl ctrlclient.Client,
	opts *flags.Options, httpclient *client.HTTPClient, totreplicas int32) (successCount, errorCount int32, err error) {
	svclb := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: setup.DeploymentName + "lb", Namespace: setup.NamespaceName},
	}

	var lbip string
	timeout, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err = wait.PollUntilContextCancel(timeout, time.Second*5, true, func(ctx context.Context) (bool, error) {
		if err := cl.Get(ctx, ctrlclient.ObjectKeyFromObject(&svclb), &svclb); err != nil {
			return false, err
		}
		if len(svclb.Status.LoadBalancer.Ingress) == 0 {
			return false, nil
		}
		lbip = svclb.Status.LoadBalancer.Ingress[0].IP
		if lbip == "" {
			lbip = svclb.Status.LoadBalancer.Ingress[0].Hostname
			if lbip == "" {
				return false, nil
			}
			hosts, err := net.LookupIP(lbip)
			if err != nil {
				return false, nil
			}
			return len(hosts) > 0, nil
		}
		return true, nil
	}); err != nil {
		return 0, 0, fmt.Errorf("failed to get load balancer IP: %w", err)
	}

	for i := 0; i < int(totreplicas*2); i++ {
		ok, err := httpclient.Curl(ctx, fmt.Sprintf("http://%s", lbip), !opts.Topts.Verbose, opts.Topts.LocalFactory.Printer.Logger)
		successCount, errorCount, err = utils.ManageResults(opts.Topts.FailFast, err, ok, successCount, errorCount)
		if err != nil {
			return successCount, errorCount, err
		}
	}

	return successCount, errorCount, nil
}

// RunsCheckExternalToLoadBalancerService runs all the checks from the external to the load balancer service.
func RunsCheckExternalToLoadBalancerService(ctx context.Context, cl *client.Client, opts *flags.Options,
	totreplicas int32) (successCountTot, errorCountTot int32, err error) {
	httpclient := client.NewHTTPClient(time.Second * 5)

	successCount, errorCount, err := RunsCheckExternalToLoadBalancerServiceWithClient(ctx, cl.Consumer, opts, httpclient, totreplicas)
	successCountTot += successCount
	errorCountTot += errorCount
	if err != nil {
		return successCount, errorCount, fmt.Errorf("consumer failed to run checks: %w", err)
	}

	for k := range cl.Providers {
		successCount, errorCount, err = RunsCheckExternalToLoadBalancerServiceWithClient(ctx, cl.Providers[k], opts, httpclient, totreplicas)
		successCountTot += successCount
		errorCountTot += errorCount
		if err != nil {
			return successCount, errorCount, fmt.Errorf("provider %q failed to run checks: %w", k, err)
		}
	}

	return successCountTot, errorCountTot, nil
}

// RunsCheckPodToNodePortService runs all the checks from the pod to the node port service.
func RunsCheckPodToNodePortService(ctx context.Context, cl *client.Client, cfg client.Configs, opts *flags.Options,
	totreplicas int32) (successCountTot, errorCountTot int32, err error) {
	successCount, errorCount, err := RunsCheckPodToNodePortServiceWithClient(ctx, cl.Consumer, cfg, opts, totreplicas, cl.ConsumerName)
	successCountTot += successCount
	errorCountTot += errorCount
	if err != nil {
		return successCount, errorCount, fmt.Errorf("consumer failed to run checks: %w", err)
	}

	for k := range cl.Providers {
		successCount, errorCount, err = RunsCheckPodToNodePortServiceWithClient(ctx, cl.Providers[k], cfg, opts, totreplicas, k)
		successCountTot += successCount
		errorCountTot += errorCount
		if err != nil {
			return successCount, errorCount, fmt.Errorf("provider %q failed to run checks: %w", k, err)
		}
	}

	return successCountTot, errorCountTot, nil
}

// RunsCheckPodToNodePortServiceWithClient runs all the checks from the pod to the node port service.
func RunsCheckPodToNodePortServiceWithClient(ctx context.Context, cl ctrlclient.Client, cfg client.Configs, opts *flags.Options,
	totreplicas int32, name string) (successCount, errorCount int32, err error) {
	svcnp := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: setup.DeploymentName + "np", Namespace: setup.NamespaceName},
	}

	if err := cl.Get(ctx, ctrlclient.ObjectKeyFromObject(&svcnp), &svcnp); err != nil {
		return 0, 0, fmt.Errorf("failed to get service: %w", err)
	}

	nodeport := svcnp.Spec.Ports[0].NodePort

	nodes := corev1.NodeList{}
	if err := cl.List(ctx, &nodes); err != nil {
		return 0, 0, fmt.Errorf("failed to list nodes: %w", err)
	}

	var successCountTot, errorCountTot int32
	for i := 0; i < int(totreplicas*2); i++ {
		for i := range nodes.Items {
			if nodes.Items[i].GetLabels()[consts.TypeLabel] == consts.TypeNode {
				continue
			}
			if opts.NodePortNodes == flags.NodePortNodesWorkers && setup.IsNodeControlPlane(nodes.Items[i].Spec.Taints) {
				continue
			}
			if opts.NodePortNodes == flags.NodePortNodesControlPlanes && !setup.IsNodeControlPlane(nodes.Items[i].Spec.Taints) {
				continue
			}
			nodeip := GetNodeAddress(&nodes.Items[i])
			successCount, errorCount, err = RunCheckToTargets(ctx, cl, cfg[name],
				opts, name, []string{fmt.Sprintf("http://%s:%d", nodeip, nodeport)}, false, ExecCurl)
			successCountTot += successCount
			errorCountTot += errorCount
			if err != nil {
				return successCountTot, errorCountTot, fmt.Errorf("consumer failed to run checks: %w", err)
			}
		}
	}

	return successCountTot, errorCountTot, nil
}
