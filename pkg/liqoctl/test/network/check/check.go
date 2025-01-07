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

	"github.com/pterm/pterm"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
)

// RunChecks runs all the checks.
func RunChecks(ctx context.Context, cl *client.Client, cfg client.Configs, opts *flags.Options, totreplicas int32) error {
	logger := opts.Topts.LocalFactory.Printer.Logger
	var successCountTot, errorCountTot int32

	logger.Info("Running checks pod to pod")
	successCount, errorCount, err := RunChecksPodToPod(ctx, cl, cfg, opts, totreplicas)
	PrintCheckResults(successCount, errorCount, logger)
	if err != nil {
		return fmt.Errorf("failed to run checks pod to pod: %w", err)
	}
	successCountTot += successCount
	errorCountTot += errorCount

	if opts.Basic {
		return nil
	}

	logger.Info("Running checks pod to service")
	successCount, errorCount, err = RunCheckPodToClusterIPService(ctx, cl, cfg, opts, totreplicas)
	PrintCheckResults(successCount, errorCount, logger)
	if err != nil {
		return fmt.Errorf("failed to run checks pod to service: %w", err)
	}
	successCountTot += successCount
	errorCountTot += errorCount

	logger.Info("Running checks node to pod")
	successCount, errorCount, err = RunChecksNodeToPod(ctx, cl, cfg, opts, totreplicas)
	PrintCheckResults(successCount, errorCount, logger)
	if err != nil {
		return fmt.Errorf("failed to run checks node to pod: %w", err)
	}
	successCountTot += successCount
	errorCountTot += errorCount

	if opts.PodToNodePort {
		logger.Info("Running checks pod to nodeport")
		successCount, errorCount, err = RunsCheckPodToNodePortService(ctx, cl, cfg, opts, totreplicas)
		PrintCheckResults(successCount, errorCount, logger)
		if err != nil {
			return fmt.Errorf("failed to run checks pod to nodeport: %w", err)
		}
		successCountTot += successCount
		errorCountTot += errorCount
	}

	if opts.NodePortExt {
		logger.Info("Running checks external to nodeport service")
		successCount, errorCount, err = RunsCheckExternalToNodePortService(ctx, cl, opts, totreplicas)
		PrintCheckResults(successCount, errorCount, logger)
		if err != nil {
			return fmt.Errorf("failed to run checks external to nodeport service: %w", err)
		}
		successCountTot += successCount
		errorCountTot += errorCount
	}

	if opts.LoadBalancer {
		logger.Info("Running checks external to loadbalancer service")
		successCount, errorCount, err = RunsCheckExternalToLoadBalancerService(ctx, cl, opts, totreplicas)
		PrintCheckResults(successCount, errorCount, logger)
		if err != nil {
			return fmt.Errorf("failed to run checks external to loadbalancer service: %w", err)
		}
		successCountTot += successCount
		errorCountTot += errorCount
	}

	logger.Info("Running checks pod to external")
	successCount, errorCount, err = RunChecksPodToExternal(ctx, cl, cfg, opts)
	PrintCheckResults(successCount, errorCount, logger)
	if err != nil {
		return fmt.Errorf("failed to run checks pod to external: %w", err)
	}
	successCountTot += successCount
	errorCountTot += errorCount

	if opts.IPRemapping {
		logger.Info("Running checks pod to external remapped IP")
		successCount, errorCount, err := RunChecksPodToExternalRemappedIP(ctx, cl, cfg, opts)
		PrintCheckResults(successCount, errorCount, logger)
		if err != nil {
			return fmt.Errorf("failed to run checks pod to external remapped IP: %w", err)
		}
		successCountTot += successCount
		errorCountTot += errorCount
	}

	logger.Info("All checks completed")
	PrintCheckResults(successCountTot, errorCountTot, logger)

	if errorCountTot > 0 {
		return fmt.Errorf("some checks failed")
	}
	return nil
}

// InitClientSet initializes the clientset.
func InitClientSet(cfg *rest.Config) (*kubernetes.Clientset, error) {
	// Create Clientset from Config
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	return clientset, nil
}

// PrintCheckResults prints the check results.
func PrintCheckResults(successCount, errorCount int32, logger *pterm.Logger) {
	if successCount > 0 {
		logger.Info("Checks succeeded", logger.Args("counter", successCount))
	}
	if errorCount > 0 {
		logger.Error("Checks failed", logger.Args("counter", errorCount))
	}
}
