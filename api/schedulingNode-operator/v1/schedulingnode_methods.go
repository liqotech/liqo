package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"strings"
)

// we'd like to have crd cluster-scoped, but there is an issue in that. Additional investigation required
var (
	domain = "drone.com"
)

func (sn *SchedulingNode) InitSchedulingNode(name string) {
	sn.Name = name
	sn.Labels = make(map[string]string)
	sn.Annotations = make(map[string]string)
	sn.Spec.ResourceQuota = corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{}}
}

func (sn *SchedulingNode) UpdateFromNode(node corev1.Node) error {

	sn.copyAnnotationsMatching(node.Labels, domain)
	sn.fillQuotas(node.Status.Allocatable)

	return nil
}

func (sn *SchedulingNode) CreateFromNode(node corev1.Node) error {
	sn.InitSchedulingNode(node.Name)
	sn.Spec.NodeName = corev1.ResourceName(node.Name)

	if t, ok := node.Labels["type"]; ok && t == "virtual-node" {
		sn.Spec.NodeType = "liqo.io/virtual"
	} else {
		sn.Spec.NodeType = "liqo.io/physical"
	}

	sn.copyAnnotationsMatching(node.Annotations, domain)
	sn.fillQuotas(node.Status.Allocatable)

	sn.Spec.LimitRange.Limits = make([]corev1.LimitRangeItem, 0)

	return nil
}

func (sn *SchedulingNode) GetNodeName() string {
	return strings.Replace(sn.Name, "."+domain, "", -1)
}

func (sn *SchedulingNode) copyAnnotationsMatching(annotations map[string]string, s string) {

	sn.Spec.Properties = make(map[corev1.ResourceName]string)

	for k, v := range annotations {
		if strings.Contains(k, s) {
			sn.Spec.Properties[corev1.ResourceName(k)] = v
		}
	}
}

func (sn *SchedulingNode) fillQuotas(allocatables corev1.ResourceList) {

	for k, v := range allocatables {
		resName := corev1.ResourceName(domain+"/") + k
		if sn.Spec.ResourceQuota.Hard == nil {
			sn.Spec.ResourceQuota.Hard = corev1.ResourceList{}
		}

		sn.Spec.ResourceQuota.Hard[resName] = v
	}
}

func CreateNamespacedName(nodeName string) types.NamespacedName {
	return types.NamespacedName{
		Name: strings.Join([]string{nodeName, domain}, "."),
	}
}
