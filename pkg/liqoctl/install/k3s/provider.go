package k3s

import (
	"context"
	"fmt"
	"net"
	"net/url"

	flag "github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

const (
	providerPrefix = "k3s"

	defaultPodCIDR     = "10.42.0.0/16"
	defaultServiceCIDR = "10.43.0.0/16"
)

var (
	localhostValues = []string{
		"localhost",
		"127.0.0.1",
	}
)

type k3sProvider struct {
	k8sClient kubernetes.Interface
	config    *rest.Config

	apiServer   string
	serviceCIDR string
	podCIDR     string

	clusterLabels map[string]string
}

// NewProvider initializes a new K3S provider struct.
func NewProvider() provider.InstallProviderInterface {
	return &k3sProvider{
		clusterLabels: map[string]string{
			consts.ProviderClusterLabel: providerPrefix,
		},
	}
}

// ValidateCommandArguments validates specific arguments passed to the install command.
func (k *k3sProvider) ValidateCommandArguments(flags *flag.FlagSet) (err error) {
	k.podCIDR, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "pod-cidr")
	if err != nil {
		return err
	}
	klog.V(3).Infof("K3S PodCIDR: %v", k.podCIDR)

	k.serviceCIDR, err = installutils.CheckStringFlagIsSet(flags, providerPrefix, "service-cidr")
	if err != nil {
		return err
	}
	klog.V(3).Infof("K3S ServiceCIDR: %v", k.serviceCIDR)

	k.apiServer, err = flags.GetString(installutils.PrefixedName(providerPrefix, "api-server"))
	if err != nil {
		return err
	}
	if k.apiServer != "" {
		klog.V(3).Infof("K3S API Server: %v", k.apiServer)
	}

	return nil
}

// ExtractChartParameters fetches the parameters used to customize the Liqo installation on a specific cluster of a
// given provider.
func (k *k3sProvider) ExtractChartParameters(ctx context.Context, config *rest.Config, _ *provider.CommonArguments) error {
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Unable to create client: %s", err)
		return err
	}

	k.k8sClient = k8sClient
	k.config = config

	if k.apiServer == "" {
		k.apiServer = config.Host
	}

	if err = k.validateAPIServer(); err != nil {
		return err
	}

	if err = k.validatePodCIDR(ctx); err != nil {
		return err
	}

	if err = k.validateServiceCIDR(ctx); err != nil {
		return err
	}

	return nil
}

// UpdateChartValues patches the values map with the values required for the selected cluster.
func (k *k3sProvider) UpdateChartValues(values map[string]interface{}) {
	values["apiServer"] = map[string]interface{}{
		"address": k.apiServer,
	}
	values["networkManager"] = map[string]interface{}{
		"config": map[string]interface{}{
			"serviceCIDR": k.serviceCIDR,
			"podCIDR":     k.podCIDR,
		},
	}
	values["discovery"] = map[string]interface{}{
		"config": map[string]interface{}{
			"clusterLabels": installutils.GetInterfaceMap(k.clusterLabels),
		},
	}
}

// GenerateFlags generates the set of specific subpath and flags are accepted for a specific provider.
func GenerateFlags(flags *flag.FlagSet) {
	subFlag := flag.NewFlagSet(providerPrefix, flag.ExitOnError)
	subFlag.SetNormalizeFunc(func(f *flag.FlagSet, name string) flag.NormalizedName {
		return flag.NormalizedName(installutils.PrefixedName(providerPrefix, name))
	})

	subFlag.String("pod-cidr", defaultPodCIDR, "The Pod CIDR for your cluster (optional)")
	subFlag.String("service-cidr", defaultServiceCIDR, "The Service CIDR for your cluster (optional)")
	subFlag.String("api-server", "", "Your cluster API Server URL (optional)")

	flags.AddFlagSet(subFlag)
}

// validateServiceCIDR validates that the services in the target cluster matches the provided service CIDR.
// This is necessary since k3s does not provide us an API to retrieve the service CIDR of the cluster. We can
// only check if the provided one fits the services IPs.
func (k *k3sProvider) validateServiceCIDR(ctx context.Context) error {
	svcList, err := k.k8sClient.CoreV1().Services(v1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	_, svcNet, err := net.ParseCIDR(k.serviceCIDR)
	if err != nil {
		return err
	}

	for i := range svcList.Items {
		svc := &svcList.Items[i]
		clusterIP := svc.Spec.ClusterIP
		if clusterIP != "None" && clusterIP != "" && !svcNet.Contains(net.ParseIP(clusterIP)) {
			klog.V(4).Infof("the service CIDR %v does not contain IP %v of service %v/%v",
				k.serviceCIDR, clusterIP, svc.GetNamespace(), svc.GetName())
			return fmt.Errorf(
				"it seems that the specified service CIDR (%v) is not correct as it does not match the services in your cluster", k.serviceCIDR)
		}
	}

	return nil
}

// validatePodCIDR validates that the pods in the target cluster matches the provided pod CIDR.
// This is necessary since k3s does not provide us an API to retrieve the pod CIDR of the cluster. We can
// only check if the provided one fits the pods IPs (excluding the HostNetwork Pods and the offloaded ones).
func (k *k3sProvider) validatePodCIDR(ctx context.Context) error {
	podList, err := k.k8sClient.CoreV1().Pods(v1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("!%v", consts.LocalPodLabelKey),
	})
	if err != nil {
		return err
	}

	_, podNet, err := net.ParseCIDR(k.podCIDR)
	if err != nil {
		return err
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		podIP := pod.Status.PodIP
		if podIP != "" && !pod.Spec.HostNetwork && !podNet.Contains(net.ParseIP(podIP)) {
			klog.V(4).Infof("the pod CIDR %v does not contain IP %v of pod %v/%v",
				k.serviceCIDR, podIP, pod.GetNamespace(), pod.GetName())
			return fmt.Errorf(
				"it seems that the specified pod CIDR (%v) is not correct as it does not match the pods in your cluster", k.podCIDR)
		}
	}

	return nil
}

func (k *k3sProvider) validateAPIServer() error {
	apiServerURL, err := url.Parse(k.apiServer)
	if err != nil {
		return err
	}

	hostname := apiServerURL.Hostname()
	for i := range localhostValues {
		if hostname == localhostValues[i] {
			return fmt.Errorf("cannot use localhost (%v) as API Server address", hostname)
		}
	}

	return nil
}
