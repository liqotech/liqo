package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
)

type RegistryType struct {
	SingularType reflect.Type
	PluralType   reflect.Type
}

var Registry = make(map[string]RegistryType)

func AddToRegistry(api string, singular, plural runtime.Object) {
	Registry[api] = RegistryType{
		SingularType: reflect.TypeOf(singular).Elem(),
		PluralType:   reflect.TypeOf(plural).Elem(),
	}
}
