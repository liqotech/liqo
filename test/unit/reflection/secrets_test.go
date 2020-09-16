package reflection

import (
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestSecretAdd(t *testing.T) {
	secretsReflector := InitTest("secrets")

	secret := v1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Data: map[string][]byte{
			"thesecret": []byte("ILoveLiqo"),
		},
		Type: "Opaque",
	}

	postadd := secretsReflector.PreProcessAdd(&secret).(*v1.Secret)

	assert.Equal(t, postadd.Namespace, "test")
}

func TestSecretUpdate(t *testing.T) {
	secretsReflector := InitTest("secrets")

	secret := v1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Data: map[string][]byte{
			"thesecret": []byte("ILoveLiqo"),
		},
		Type: "Opaque",
	}

	postadd := secretsReflector.PreProcessAdd(&secret).(*v1.Secret)

	assert.Equal(t, postadd.Namespace, "test")

}
