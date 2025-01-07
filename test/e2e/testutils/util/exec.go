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

package util

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	remotecommandclient "k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
)

// ExecLiqoctl runs a liqoctl command targeting the cluster specified by the given kubeconfig.
func ExecLiqoctl(kubeconfig string, args []string, output io.Writer) error {
	liqoctl := os.Getenv("LIQOCTL")
	if liqoctl == "" {
		return errors.New("failed to retrieve liqoctl executable")
	}

	//nolint:gosec // running in a trusted environment
	cmd := exec.Command(liqoctl, args...)
	cmd.Stdout = output
	cmd.Stderr = output
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
	return cmd.Run()
}

// ExecCmd executes a command inside a pod.
func ExecCmd(ctx context.Context, config *rest.Config, client kubernetes.Interface,
	podName, namespace, command string) (stdOut, stdErr string, retErr error) {
	cmd := []string{
		"sh",
		"-c",
		command,
	}
	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")
	req.VersionedParams(&v1.PodExecOptions{
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
		Command: cmd,
	}, scheme.ParameterCodec)

	var stdout, stderr bytes.Buffer
	executor, err := remotecommandclient.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", err
	}
	err = executor.StreamWithContext(ctx, remotecommandclient.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	return stdout.String(), stderr.String(), err
}

// TriggerCheckNodeConnectivity checks nodePort service connectivity, executing a command for every node in the target cluster.
func TriggerCheckNodeConnectivity(localNodes *v1.NodeList, command string, nodePortValue int) error {
	if nodePortValue <= 0 {
		return fmt.Errorf("nodePort Value invalid (Must be >= 0)")
	}
	for index := range localNodes.Items {
		if len(localNodes.Items) != 1 && IsNodeControlPlane(localNodes.Items[index].Spec.Taints) {
			continue
		}
		cmd := command + localNodes.Items[index].Status.Addresses[0].Address + ":" + strconv.Itoa(nodePortValue)
		c := exec.Command("sh", "-c", cmd) //nolint:gosec // Just a test, no need for this check
		output := &bytes.Buffer{}
		errput := &bytes.Buffer{}
		c.Stdout = output
		c.Stderr = errput
		klog.Infof("running command: %s", cmd)
		err := c.Run()
		if err != nil {
			klog.Error(err)
			klog.Info(output.String())
			klog.Info(errput.String())
			return err
		}
	}
	return nil
}
