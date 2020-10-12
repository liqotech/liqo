package labelPolicy

import corev1 "k8s.io/api/core/v1"

type LabelPolicyType string

// NOTE: add these values to the accepted values in apis/config/v1alpha1/clusterconfig_types.go > LabelPolicy > Policy
const (
	// add val=true label if at least one node has a val=true or val="" label
	LabelPolicyAnyTrue LabelPolicyType = "LabelPolicyAnuTrue"
	// add val=true label if each node has a val=true or val="" label
	LabelPolicyAllTrue LabelPolicyType = "LabelPolicyAllTrue"
	// add val="" label if at least one node has a val=true or val="" label
	LabelPolicyAnyTrueNoLabelIfFalse LabelPolicyType = "LabelPolicyAnyTrueNoLabelIfFalse"
	// add val="" label if each node has a val=true or val="" label
	LabelPolicyAllTrueNoLabelIfFalse LabelPolicyType = "LabelPolicyAllTrueNoLabelIfFalse"
)

type LabelPolicy interface {
	Process(physicalNodes *corev1.NodeList, key string) (value string, insertLabel bool)
}

func GetInstance(policyType LabelPolicyType) LabelPolicy {
	switch policyType {
	case LabelPolicyAnyTrue:
		return &AnyTrue{}
	case LabelPolicyAllTrue:
		return &AllTrue{}
	case LabelPolicyAnyTrueNoLabelIfFalse:
		return &AnyTrueNoLabelIfFalse{}
	case LabelPolicyAllTrueNoLabelIfFalse:
		return &AllTrueNoLabelIfFalse{}
	default:
		return &AnyTrue{}
	}
}
