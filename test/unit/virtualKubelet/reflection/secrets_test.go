package reflection

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	api "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/outgoing"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping/test"
	storageTest "github.com/liqotech/liqo/pkg/virtualKubelet/storage/test"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestSecretAdd(t *testing.T) {
	foreignClient := fake.NewSimpleClientset()
	cacheManager := &storageTest.MockManager{
		HomeCache:    map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
		ForeignCache: map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
	}
	nattingTable := &test.MockNamespaceMapper{Cache: map[string]string{}}

	Greflector := &api.GenericAPIReflector{
		ForeignClient:    foreignClient,
		NamespaceNatting: nattingTable,
		CacheManager:     cacheManager,
	}

	reflector := &outgoing.SecretsReflector{
		APIReflector: Greflector,
	}
	reflector.SetSpecializedPreProcessingHandlers()

	secret := v1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "homeNamespace",
		},
		Data: map[string][]byte{
			"thesecret": []byte("ILoveLiqo"),
		},
		Type: "Opaque",
	}

	_, _ = nattingTable.NatNamespace("homeNamespace", true)
	postadd := reflector.PreProcessAdd(&secret).(*v1.Secret)

	assert.Equal(t, postadd.Namespace, "homeNamespace-natted")
}

func TestSASecretAdd(t *testing.T) {
	foreignClient := fake.NewSimpleClientset()
	cacheManager := &storageTest.MockManager{
		HomeCache:    map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
		ForeignCache: map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
	}
	nattingTable := &test.MockNamespaceMapper{Cache: map[string]string{}}

	Greflector := &api.GenericAPIReflector{
		ForeignClient:    foreignClient,
		NamespaceNatting: nattingTable,
		CacheManager:     cacheManager,
	}

	reflector := &outgoing.SecretsReflector{
		APIReflector: Greflector,
	}
	reflector.SetSpecializedPreProcessingHandlers()

	secret := v1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "homeNamespace",
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

	_, _ = nattingTable.NatNamespace("homeNamespace", true)
	postadd := reflector.PreProcessAdd(&secret).(*v1.Secret)

	assert.Equal(t, postadd.Namespace, "homeNamespace-natted")
	assert.Assert(t, len(postadd.Annotations) == 0, "service account annotation are not removed")
	assert.Equal(t, postadd.Type, v1.SecretTypeOpaque)
	assert.Equal(t, postadd.Labels["kubernetes.io/service-account.name"], "test-sa", "service account reference label is not set correctly")
}

func TestSecretUpdate(t *testing.T) {
	foreignClient := fake.NewSimpleClientset()
	cacheManager := &storageTest.MockManager{
		HomeCache:    map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
		ForeignCache: map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
	}
	nattingTable := &test.MockNamespaceMapper{Cache: map[string]string{}}

	Greflector := &api.GenericAPIReflector{
		ForeignClient:    foreignClient,
		NamespaceNatting: nattingTable,
		CacheManager:     cacheManager,
	}

	reflector := &outgoing.SecretsReflector{
		APIReflector: Greflector,
	}
	reflector.SetSpecializedPreProcessingHandlers()

	secret := v1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "homeNamespace",
		},
		Data: map[string][]byte{
			"thesecret": []byte("ILoveLiqo"),
		},
		Type: "Opaque",
	}

	_, _ = nattingTable.NatNamespace("homeNamespace", true)
	postadd := reflector.PreProcessAdd(&secret).(*v1.Secret)

	assert.Equal(t, postadd.Namespace, "homeNamespace-natted")
}
