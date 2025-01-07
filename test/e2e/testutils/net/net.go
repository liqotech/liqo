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

package net

import (
	"context"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
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
	command            = "timeout 15 curl --retry 60 --fail --max-time 2 -s -o /dev/null -w '%{http_code}' "
)

func init() {
	// get the DOCKER_PROXY variable from the environment, if set.
	dockerProxy, ok := os.LookupEnv("DOCKER_PROXY")
	if ok {
		image = dockerProxy + "/" + image
	}
}

// ConnectivityCheckNodeToPod creates a NodePort Service and check its availability.
func ConnectivityCheckNodeToPod(ctx context.Context, homeClusterClient kubernetes.Interface,
	clusterID liqov1beta1.ClusterID, remotePodName string) error {
	nodePort, err := EnsureNodePortService(ctx, homeClusterClient, remotePodName)
	if err != nil {
		return err
	}
	return CheckNodeToPortConnectivity(ctx, homeClusterClient, clusterID, nodePort)
}

// EnsureNodePortService creates a nodePortService. It returns the port to contact to reach the service and occurred errors.
func EnsureNodePortService(ctx context.Context, homeClusterClient kubernetes.Interface, remotePodName string) (int, error) {
	nodePort, err := EnsureNodePort(ctx, homeClusterClient, remotePodName, TestNamespaceName)
	if err != nil {
		return 0, err
	}
	return int(nodePort.Spec.Ports[0].NodePort), nil
}

// CheckNodeToPortConnectivity contacts the nodePortValue and returns the result.
func CheckNodeToPortConnectivity(ctx context.Context, homeClusterClient kubernetes.Interface,
	homeClusterID liqov1beta1.ClusterID, nodePortValue int) error {
	localNodes, err := util.GetNodes(ctx, homeClusterClient, homeClusterID, labelSelectorNodes)
	if err != nil {
		return err
	}
	return util.TriggerCheckNodeConnectivity(localNodes, command, nodePortValue)
}

// CheckPodConnectivity contacts the remote pod by executing the command inside podRemoteUpdateCluster1.
func CheckPodConnectivity(ctx context.Context,
	homeConfig *restclient.Config, homeClient kubernetes.Interface, cluster1PodName, cluster2PodName string) error {
	clientPod, serverPod, err := getPods(ctx, homeClient, cluster1PodName, cluster2PodName)
	if err != nil {
		klog.Error(err)
		return err
	}
	cmd := command + serverPod.Status.PodIP
	return execCmd(ctx, homeConfig, homeClient, clientPod, cmd)
}

// CheckServiceConnectivity contacts the remote service by executing the command inside podRemoteUpdateCluster1.
func CheckServiceConnectivity(ctx context.Context,
	homeConfig *restclient.Config, homeClient kubernetes.Interface, cluster1PodName, cluster2PodName string) error {
	clientPod, serverPod, err := getPods(ctx, homeClient, cluster1PodName, cluster2PodName)
	if err != nil {
		klog.Error(err)
		return err
	}
	cmd := command + serverPod.GetName()
	return execCmd(ctx, homeConfig, homeClient, clientPod, cmd)
}

func getPods(ctx context.Context, homeClient kubernetes.Interface,
	cluster1PodName, cluster2PodName string) (clientPod, serverPod *v1.Pod, err error) {
	clientPod, err = homeClient.CoreV1().Pods(TestNamespaceName).Get(ctx, cluster1PodName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	serverPod, err = homeClient.CoreV1().Pods(TestNamespaceName).Get(ctx, cluster2PodName, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return nil, nil, err
	}
	return clientPod, serverPod, nil
}

func execCmd(ctx context.Context, homeConfig *restclient.Config, homeClient kubernetes.Interface, clientPod *v1.Pod, cmd string) error {
	klog.Infof("running command %s", cmd)
	stdout, stderr, err := util.ExecCmd(ctx, homeConfig, homeClient, clientPod.Name, clientPod.Namespace, cmd)
	klog.Infof("stdout: %s", stdout)
	klog.Infof("stderr: %s", stderr)
	if err != nil {
		return err
	}
	if stdout != "200" {
		return fmt.Errorf("the stdout value (%v) is different from the expected value (200)", stdout)
	}
	return nil
}
