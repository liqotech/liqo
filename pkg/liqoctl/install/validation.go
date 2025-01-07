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

package install

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"slices"
	"strings"

	"github.com/goombaio/namegenerator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// validate validates the correctness of the different parameters.
func (o *Options) validate(ctx context.Context) error {
	if err := o.validateClusterID(); err != nil {
		return fmt.Errorf("failed validating cluster id: %w", err)
	}
	o.Printer.Verbosef("Cluster id: %s\n", o.ClusterID)

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

	if err := o.validateOutputValues(); err != nil {
		return fmt.Errorf("failed validating output values: %w", err)
	}

	return nil
}

// validateClusterID validates the provided cluster id, and generates it if necessary.
func (o *Options) validateClusterID() (err error) {
	// If at this point the name is still empty, generate a new cluster name
	if o.ClusterID == "" {
		randomName := namegenerator.NewNameGenerator(rand.Int63()).Generate() //nolint:gosec // don't need crypto/rand
		o.ClusterID = liqov1beta1.ClusterID(strings.ReplaceAll(randomName, "_", "-"))
		o.Printer.Info.Printf("No cluster id specified. Generated: %q", o.ClusterID)
	}

	errs := validation.IsDNS1123Label(string(o.ClusterID))
	if len(errs) != 0 {
		return fmt.Errorf(`the cluster name must be DNS-compatible (e.g., lowercase letters, numbers and hyphens) and
		must be <= 63 characters. Try using 'liqoctl --cluster-id' to set another name and overcome this error`)
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

		if !slices.Contains(localhostValues, apiServerURL.Hostname()) {
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

	if slices.Contains(localhostValues, apiServerURL.Hostname()) {
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

// validateOutputValues validates the flags related to values.
func (o *Options) validateOutputValues() (err error) {
	if o.ValuesPath != "" && !o.OnlyOutputValues {
		return fmt.Errorf("--dump-values-path can only be used in conjunction with --only-output-values")
	}
	if o.ValuesPath == "" {
		o.ValuesPath = DefaultDumpValuesPath
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
