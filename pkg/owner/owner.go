package owner

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func GetOwnerByKind(ownerReferences *[]v1.OwnerReference, kind string) *v1.OwnerReference {
	for _, or := range *ownerReferences {
		if or.Kind == kind {
			return &or
		}
	}
	return nil
}
