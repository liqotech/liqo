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

package pod

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

// ExecInPod executes a command in a pod.
func ExecInPod(ctx context.Context, clset *kubernetes.Clientset, cfg *rest.Config,
	pod *corev1.Pod, cmd string) (stdout, stderr string, err error) {
	// Prepare the API URL used to execute the command
	url := clset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command:   strings.Split(cmd, " "),
			Container: pod.Spec.Containers[0].Name,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec).URL()

	// Execute the command
	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", url)
	if err != nil {
		return "", "", fmt.Errorf("failed to initialize command executor: %w", err)
	}

	// Capture the output and error streams
	var stdoutBuff, stderrBuff bytes.Buffer
	if err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdoutBuff,
		Stderr: &stderrBuff,
		Tty:    false,
	}); err != nil {
		return "", "", fmt.Errorf("failed to execute command: %w", err)
	}

	return stdoutBuff.String(), stderrBuff.String(), nil
}

// TryFor tries to execute the function f for a maximum of maxRetries times.
func TryFor(ctx context.Context, maxRetries int, f func() (bool, error)) (bool, error) {
	var err error
	var ok bool
	for i := 0; i < maxRetries; i++ {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		ok, err = f()
		if ok {
			return ok, nil
		}
	}
	return ok, err
}
