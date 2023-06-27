package virtualkubelet

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// Lister is an interface for listers.
type Lister[T any] interface {
	List(selector labels.Selector) (ret []T, err error)
}

// ImplementList returns a list of NamespacedName objects from the given listers.
func ImplementList[T Lister[O], O metav1.Object](listers map[string]T) ([]any, error) {
	var err error
	objs := map[string][]O{}
	tot := 0
	for k, l := range listers {
		objs[k], err = l.List(labels.Everything())
		if err != nil {
			return nil, err
		}
		tot += len(objs[k])
	}
	list := make([]any, tot)
	i := 0
	for k := range listers {
		for j := range objs[k] {
			list[i] = forgeNamespacedName(objs[k][j])
			i++
		}
	}
	return list, nil
}

// ForgeNamespacedName returns a NamespacedName object from the given object.
func forgeNamespacedName(src metav1.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: src.GetNamespace(),
		Name:      src.GetName(),
	}
}
