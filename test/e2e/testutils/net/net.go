// Copyright 2019-2021 The Liqo Authors
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

package net

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

var (
	image             = "nginx"
	podTesterLocalCl  = "tester-local"
	podTesterRemoteCl = "tester-remote"
	// TestNamespaceName is the namespace name where the test is performed.
	TestNamespaceName = "test-connectivity"
	// label to list only the real nodes excluding the virtual ones.
	labelSelectorNodes = fmt.Sprintf("%v!=%v", liqoconst.TypeLabel, liqoconst.TypeNode)
	command            = "curl --retry 60 --fail --max-time 2 -s -o /dev/null -w '%{http_code}' "
)

// ConnectivityCheckNodeToPod creates a NodePort Service and check its availability.
func ConnectivityCheckNodeToPod(ctx context.Context, homeClusterClient kubernetes.Interface, clusterID string) error {
	nodePort, err := EnsureNodePortService(ctx, homeClusterClient, clusterID)
	if err != nil {
		return err
	}
	return CheckNodeToPortConnectivity(ctx, homeClusterClient, clusterID, nodePort)
}

// EnsureNodePortService creates a nodePortService. It returns the port to contact to reach the service and occurred errors.
func EnsureNodePortService(ctx context.Context, homeClusterClient kubernetes.Interface, clusterID string) (int, error) {
	nodePort, err := EnsureNodePort(ctx, homeClusterClient, clusterID, podTesterRemoteCl, TestNamespaceName)
	if err != nil {
		return 0, err
	}
	return int(nodePort.Spec.Ports[0].NodePort), nil
}

// CheckNodeToPortConnectivity contacts the nodePortValue and returns the result.
func CheckNodeToPortConnectivity(ctx context.Context, homeClusterClient kubernetes.Interface, homeClusterID string, nodePortValue int) error {
	localNodes, err := util.GetNodes(ctx, homeClusterClient, homeClusterID, labelSelectorNodes)
	if err != nil {
		return err
	}
	return util.TriggerCheckNodeConnectivity(localNodes, command, nodePortValue)
}

// CheckPodConnectivity contacts the remote service by executing the command inside podRemoteUpdateCluster1.
func CheckPodConnectivity(ctx context.Context, homeConfig *restclient.Config, homeClient kubernetes.Interface) error {
	podLocalUpdate, err := homeClient.CoreV1().Pods(TestNamespaceName).Get(ctx, podTesterLocalCl, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	podRemoteUpdateCluster1, err := homeClient.CoreV1().Pods(TestNamespaceName).Get(ctx, podTesterRemoteCl, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	cmd := command + podRemoteUpdateCluster1.Status.PodIP
	klog.Infof("running command %s", cmd)
	stdout, stderr, err := util.ExecCmd(homeConfig, homeClient, podLocalUpdate.Name, podLocalUpdate.Namespace, cmd)
	if stdout == "200" && err == nil {
		return nil
	}
	klog.Infof("stdout: %s", stderr)
	klog.Infof("stderr: %s", stderr)
	return err
}
