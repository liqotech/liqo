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
	"strings"

	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
	testutils "github.com/liqotech/liqo/pkg/liqoctl/test/utils"
	podutils "github.com/liqotech/liqo/pkg/liqoctl/utils/pod"
)

// MaxRetries is the maximum number of retries for the command.
const MaxRetries = 10

// ExecFunc is the function to execute.
type ExecFunc func(ctx context.Context, pod *corev1.Pod, clset *kubernetes.Clientset,
	cfg *rest.Config, quiet bool, endpoint string, logger *pterm.Logger) (ok bool, err error)

// RunCheckToTargets runs the checks to the targets.
func RunCheckToTargets(ctx context.Context, cl ctrlclient.Client, cfg *rest.Config, opts *flags.Options,
	owner string, targets []string, hostnetwork bool, execFunc ExecFunc) (successCount, errorCount int32, err error) {
	logger := opts.Topts.LocalFactory.Printer.Logger
	pods, err := listPods(ctx, cl, owner, hostnetwork)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list pods: %w", err)
	}

	clset, err := InitClientSet(cfg)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to initialize clientset: %w", err)
	}
	for i := range pods.Items {
		for j := range targets {
			ok, err := podutils.TryFor(ctx, MaxRetries, func() (bool, error) {
				return execFunc(ctx, &pods.Items[i], clset, cfg, !opts.Topts.Verbose, targets[j], logger)
			})
			if !ok || err != nil {
				logger.Error(fmt.Sprintf("Curl command failed after %d retries", MaxRetries), logger.Args(
					"pod", pods.Items[i].Name, "target", targets[j], "error", err,
				))
			}
			successCount, errorCount, err = testutils.ManageResults(opts.Topts.FailFast, err, ok, successCount, errorCount)
			if err != nil {
				return successCount, errorCount, err
			}
		}
	}
	return successCount, errorCount, nil
}

// ExecCurl executes a curl command.
func ExecCurl(ctx context.Context, pod *corev1.Pod, clset *kubernetes.Clientset,
	cfg *rest.Config, quiet bool, endpoint string, logger *pterm.Logger) (ok bool, err error) {
	cmd := fmt.Sprintf("curl -k --connect-timeout 5 -I %s", endpoint)
	stdout, stderr, err := podutils.ExecInPod(ctx, clset, cfg, pod, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to execute curl command: %w", err)
	}

	// Check if the curl command was successful
	if strings.Contains(stdout, "200 OK") ||
		strings.Contains(stdout, "HTTP/2 200") ||
		strings.Contains(stdout, "301 Moved Permanently") {
		if !quiet {
			logger.Info("Curl command successful", logger.Args(
				"pod", pod.Name, "target", endpoint,
			))
		}
		return true, nil
	}

	logger.Warn("Curl command failed", logger.Args(
		"pod", pod.Name, "target", endpoint, "stderr", stderr, "error", err,
	))

	return false, nil
}

// ExecNetcatTCPConnect executes a netcat command.
func ExecNetcatTCPConnect(ctx context.Context, pod *corev1.Pod,
	clset *kubernetes.Clientset, cfg *rest.Config, quiet bool, endpoint string, logger *pterm.Logger) (ok bool, err error) {
	cmd := fmt.Sprintf("nc -z %s 443", endpoint)
	_, stderr, err := podutils.ExecInPod(ctx, clset, cfg, pod, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to execute curl command: %w", err)
	}

	// Check if the curl command was successful
	if strings.Contains(stderr, "succeeded") {
		if !quiet {
			logger.Info("Netcat connection successful", logger.Args(
				"pod", pod.Name, "target", endpoint,
			))
		}
		return true, nil
	}

	logger.Warn("Netcat connection failed", logger.Args(
		"pod", pod.Name, "target", endpoint, "stderr", stderr, "error", err,
	))

	return false, nil
}
