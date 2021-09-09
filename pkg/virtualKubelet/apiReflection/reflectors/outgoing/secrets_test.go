// Copyright 2019-2021 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package outgoing

import (
	"testing"

	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	api "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping/test"
	storageTest "github.com/liqotech/liqo/pkg/virtualKubelet/storage/test"
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

	reflector := &SecretsReflector{
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

	nattingTable.NewNamespace("homeNamespace")

	pa, _ := reflector.PreProcessAdd(&secret)
	postadd := pa.(*v1.Secret)

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

	reflector := &SecretsReflector{
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

	nattingTable.NewNamespace("homeNamespace")
	pa, _ := reflector.PreProcessAdd(&secret)
	postadd := pa.(*v1.Secret)

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

	reflector := &SecretsReflector{
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

	nattingTable.NewNamespace("homeNamespace")

	pa, _ := reflector.PreProcessAdd(&secret)
	postadd := pa.(*v1.Secret)

	assert.Equal(t, postadd.Namespace, "homeNamespace-natted")
}
