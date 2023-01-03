// Copyright 2019-2023 The Liqo Authors
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

package install

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"strings"

	"github.com/goombaio/namegenerator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/slice"
)

// validate validates the correctness of the different parameters.
func (o *Options) validate(ctx context.Context) error {
	if err := o.validateClusterName(); err != nil {
		return fmt.Errorf("failed validating cluster name: %w", err)
	}
	o.Printer.Verbosef("Cluster name: %s\n", o.ClusterName)

	if err := o.validateAPIServer(); err != nil {
		return fmt.Errorf("failed validating API Server URL %q: %w", o.APIServer, err)
	}
	o.Printer.Verbosef("Kubernetes API Server: %s\n", o.APIServer)

	if err := o.validatePodCIDR(ctx); err != nil {
		return fmt.Errorf("failed validating Pod CIDR %q: %w. "+
			"Try setting the correct Pod CIDR using the vanilla *liqoctl install* command, or installing Liqo with Helm",
			o.PodCIDR, err)
	}
	o.Printer.Verbosef("Pod CIDR: %s\n", o.PodCIDR)

	if err := o.validateServiceCIDR(ctx); err != nil {
		return fmt.Errorf("failed validating Service CIDR %q: %w. "+
			"Try setting the correct Service CIDR using the vanilla *liqoctl install* command, or installing Liqo with Helm",
			o.ServiceCIDR, err)
	}
	o.Printer.Verbosef("Service CIDR: %s\n", o.ServiceCIDR)

	return nil
}

// validateClusterName validates the provided cluster name, and generates it if necessary.
func (o *Options) validateClusterName() (err error) {
	// If at this point the name is still empty, generate a new cluster name
	if o.ClusterName == "" {
		randomName := namegenerator.NewNameGenerator(rand.Int63()).Generate() //nolint:gosec // don't need crypto/rand
		o.ClusterName = strings.ReplaceAll(randomName, "_", "-")
		o.Printer.Info.Printf("No cluster name specified. Generated: %q", o.ClusterName)
	}

	errs := validation.IsDNS1123Label(o.ClusterName)
	if len(errs) != 0 {
		return fmt.Errorf("the cluster name may only contain lowercase letters, numbers and hyphens, and must not be no longer than 63 characters")
	}

	return nil
}

// validateAPIServer validates that the API server URL does not point to invalid addresses.
func (o *Options) validateAPIServer() error {
	var localhostValues = []string{
		"localhost",
		"127.0.0.1",
		"0.0.0.0", // Configured by k3d
	}

	// In case the API server URL is not set, fallback to the one specified in the REST config.
	if o.APIServer == "" {
		// Defaulting to the REST config value is disabled, hence skip all checks.
		if o.DisableAPIServerDefaulting {
			return nil
		}

		// Do not fallback to the API server URL in the REST config if it refers to localhost.
		apiServerURL, err := url.Parse(o.RESTConfig.Host)
		if err != nil {
			return err
		}

		if !slice.ContainsString(localhostValues, apiServerURL.Hostname()) {
			o.APIServer = o.RESTConfig.Host
		}

		return nil
	}

	if !o.DisableAPIServerSanityChecks {
		// Validate that the retrieve endpoint matches the one of the REST config.
		if err := o.validateAPIServerConsistency(); err != nil {
			return err
		}
	}

	apiServerURL, err := url.Parse(o.APIServer)
	if err != nil {
		return err
	}

	if slice.ContainsString(localhostValues, apiServerURL.Hostname()) {
		return fmt.Errorf("cannot use localhost as API Server address")
	}

	return nil
}

func (o *Options) validateAPIServerConsistency() error {
	configHostname, configPort, err := getHostnamePort(o.RESTConfig.Host)
	if err != nil {
		return err
	}

	endpointHostname, endpointPort, err := getHostnamePort(o.APIServer)
	if err != nil {
		return err
	}

	if configHostname != endpointHostname || configPort != endpointPort {
		msg := "the retrieved API server URL (%q) does not match the kubeconfig one (%q). If correct, skip with --disable-api-server-sanity-check"
		return fmt.Errorf(msg, o.APIServer, o.RESTConfig.Host)
	}

	return nil
}

// validatePodCIDR validates that the pods in the target cluster matche the provided Pod CIDR.
func (o *Options) validatePodCIDR(ctx context.Context) error {
	pods, err := o.KubeClient.CoreV1().Pods(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("!%v", consts.LocalPodLabelKey),
	})
	if err != nil {
		return err
	}

	_, podNet, err := net.ParseCIDR(o.PodCIDR)
	if err != nil {
		return err
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		podIP := pod.Status.PodIP
		if podIP != "" && !pod.Spec.HostNetwork && !podNet.Contains(net.ParseIP(podIP)) {
			return fmt.Errorf(
				"pod %q does not match the provided Pod CIDR (IP: %v)", pod.GetName(), podIP)
		}
	}

	return nil
}

// validateServiceCIDR validates that the services in the target cluster matche the provided Service CIDR.
func (o *Options) validateServiceCIDR(ctx context.Context) error {
	// TODO: check if limit works
	services, err := o.KubeClient.CoreV1().Services(corev1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	_, svcNet, err := net.ParseCIDR(o.ServiceCIDR)
	if err != nil {
		return err
	}

	for i := range services.Items {
		clusterIP := services.Items[i].Spec.ClusterIP
		if clusterIP != "None" && clusterIP != "" && !svcNet.Contains(net.ParseIP(clusterIP)) {
			return fmt.Errorf(
				"service %q does not match the provided Service CIDR (IP: %v)", services.Items[i].GetName(), clusterIP)
		}
	}

	return nil
}

func getHostnamePort(urlString string) (hostname, port string, err error) {
	if !strings.HasPrefix(urlString, "https://") {
		urlString = fmt.Sprintf("https://%v", urlString)
	}

	var parsedURL *url.URL
	parsedURL, err = url.Parse(urlString)
	if err != nil {
		return "", "", err
	}

	hostname = parsedURL.Hostname()
	port = parsedURL.Port()

	return hostname, defaultPort(port), nil
}

func defaultPort(port string) string {
	if port == "" {
		port = "443"
	}
	return port
}
