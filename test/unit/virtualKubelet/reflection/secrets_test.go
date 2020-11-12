package reflection

import (
	"github.com/liqotech/liqo/test/unit/virtualKubelet/utils"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestSecretAdd(t *testing.T) {
	secretsReflector := utils.InitTest("secrets")

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

func TestSASecretAdd(t *testing.T) {
	secretsReflector := utils.InitTest("secrets")

	secret := v1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": "test-sa",
				"kubernetes.io/service-account.uid":  "test-uid",
			},
		},
		Data: map[string][]byte{
			"thesecret": []byte("ILoveLiqo"),
		},
		Type: v1.SecretTypeServiceAccountToken,
	}

	postadd := secretsReflector.PreProcessAdd(&secret).(*v1.Secret)

	assert.Equal(t, postadd.Namespace, "test")
	assert.Assert(t, len(postadd.Annotations) == 0, "service account annotation are not removed")
	assert.Equal(t, postadd.Type, v1.SecretTypeOpaque)
	assert.Equal(t, postadd.Labels["kubernetes.io/service-account.name"], "test-sa", "service account reference label is not set correctly")
}

func TestSecretUpdate(t *testing.T) {
	secretsReflector := utils.InitTest("secrets")

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
