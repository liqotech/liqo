package test

import (
	v1 "github.com/liqotech/liqo/api/virtualKubelet/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateNamespaceNattingTable(foreignClusterId string) *v1.NamespaceNattingTable {
	return &v1.NamespaceNattingTable{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: foreignClusterId,
		},
		Spec: v1.NamespaceNattingTableSpec{
			ClusterId: foreignClusterId,
			NattingTable: map[string]string{
				Namespace: NattedNamespace,
			},
			DeNattingTable: map[string]string{
				NattedNamespace: Namespace,
			},
		},
	}
}
