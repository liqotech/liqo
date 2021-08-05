package kubeadm

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func extractValueFromArgumentList(argumentMatch string, argumentList []string) (string, error) {
	for index := range argumentList {
		matched, _ := regexp.Match(argumentMatch, []byte(argumentList[index]))
		if matched {
			return strings.Split(argumentList[index], "=")[1], nil
		}
	}
	return "", fmt.Errorf("argument not found")
}

func retrieveClusterParameters(ctx context.Context, client kubernetes.Interface) (podCIDR, serviceCIDR string, err error) {
	kubeControllerSpec, err := client.CoreV1().Pods(kubeSystemNamespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(kubeControllerManagerLabels).AsSelector().String(),
	})
	if err != nil {
		return "", "", err
	}
	if len(kubeControllerSpec.Items) < 1 {
		return "", "", fmt.Errorf("kube-controller-manager not found")
	}
	if len(kubeControllerSpec.Items[0].Spec.Containers) != 1 {
		return "", "", fmt.Errorf("unexpected amount of containers in kube-controller-manager")
	}
	command := kubeControllerSpec.Items[0].Spec.Containers[0].Command
	podCIDR, err = extractValueFromArgumentList(podCIDRParameterFilter, command)
	klog.V(4).Infof("Extracted podCIDR: %s\n", podCIDR)
	if err != nil {
		return "", "", err
	}
	serviceCIDR, err = extractValueFromArgumentList(serviceCIDRParameterFilter, command)
	klog.V(4).Infof("Extracted serviceCIDR: %s\n", serviceCIDR)
	if err != nil {
		return "", "", err
	}
	return podCIDR, serviceCIDR, nil
}
