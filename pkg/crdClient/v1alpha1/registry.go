package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
)

var Registry = make(map[string]reflect.Type)

func AddToRegistry(name string, o runtime.Object) {
	Registry[name] = reflect.TypeOf(o).Elem()
}
