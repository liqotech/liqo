package broadcaster

import (
	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/labelPolicy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
)

// GetNodeImages retrieves available images on a specific image
func GetNodeImages(node corev1.Node) []corev1.ContainerImage {
	m := make(map[string]corev1.ContainerImage)

	for _, image := range node.Status.Images {
		for _, name := range image.Names {
			// TODO: policy to decide which images to announce
			if !strings.Contains(name, "k8s") {
				m[name] = image
			}
		}
	}
	images := make([]corev1.ContainerImage, len(m))
	i := 0
	for _, image := range m {
		images[i] = image
		i++
	}
	return images
}

// GetLabels get labels for advertisement.
func GetLabels(physicalNodes *corev1.NodeList, labelPolicies []configv1alpha1.LabelPolicy) (labels map[string]string) {
	labels = make(map[string]string)
	if labelPolicies == nil {
		return labels
	}
	for _, lblPol := range labelPolicies {
		if val, insert := labelPolicy.GetInstance(lblPol.Policy).Process(physicalNodes, lblPol.Key); insert {
			labels[lblPol.Key] = val
		}
	}
	return labels
}

// ComputeAnnouncedResources creates announced resources for advertisement.
func ComputeAnnouncedResources(physicalNodes *corev1.NodeList, reqs corev1.ResourceList, sharingPercentage int64) (availability corev1.ResourceList, images []corev1.ContainerImage) {
	// get allocatable resources in all the physical nodes
	allocatable, images := GetClusterResources(physicalNodes.Items)

	// subtract used resources from available ones to have available resources
	availability = allocatable.DeepCopy()
	for k, v := range availability {
		if req, ok := reqs[k]; ok {
			v.Sub(req)
		}
		if v.Value() < 0 {
			v.Set(0)
		}
		if k == corev1.ResourceCPU {
			// use millis
			v.SetScaled(v.MilliValue()*sharingPercentage/100, resource.Milli)
		} else if k == corev1.ResourceMemory {
			// use mega
			v.SetScaled(v.ScaledValue(resource.Mega)*sharingPercentage/100, resource.Mega)
		} else {
			v.Set(v.Value() * sharingPercentage / 100)
		}
		availability[k] = v
	}
	return availability, images
}

// ComputePrices creates prices resource for advertisement.
func ComputePrices(images []corev1.ContainerImage) corev1.ResourceList {
	//TODO: logic to set prices
	prices := corev1.ResourceList{}
	prices[corev1.ResourceCPU] = *resource.NewQuantity(1, resource.DecimalSI)
	prices[corev1.ResourceMemory] = resource.MustParse("2m")
	for _, image := range images {
		for _, name := range image.Names {
			prices[corev1.ResourceName(name)] = *resource.NewQuantity(5, resource.DecimalSI)
		}
	}
	return prices
}

func contains(arr []configv1alpha1.LabelPolicy, el configv1alpha1.LabelPolicy) bool {
	for _, a := range arr {
		if a.Key == el.Key && a.Policy == el.Policy {
			return true
		}
	}
	return false
}

func differentLabels(current, next []configv1alpha1.LabelPolicy) bool {
	if len(current) != len(next) {
		return true
	}
	for _, l := range current {
		if !contains(next, l) {
			return true
		}
	}
	return false
}