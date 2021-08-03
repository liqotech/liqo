package serviceEnv

import (
	"sort"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubelet/envvars"
	"k8s.io/utils/pointer"

	testutil "github.com/liqotech/liqo/pkg/utils/testUtils"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/storage/test"
)

var cacheReader = &test.MockManager{
	HomeCache:    map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
	ForeignCache: map[string]map[apimgmt.ApiType]map[string]metav1.Object{},
}

var sortOpt gocmp.Option = gocmp.Transformer("Sort", sortEnv)

func sortEnv(in []corev1.EnvVar) []corev1.EnvVar {
	out := append([]corev1.EnvVar(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func TestServiceEnvVar(t *testing.T) {
	namespace := "namespace"
	namespace2 := "namespace-02"

	service1 := testutil.FakeService(metav1.NamespaceDefault, "kubernetes", "1.2.3.1", "TCP", 8081)
	service2 := testutil.FakeService(namespace, "test", "1.2.3.3", "TCP", 8083)
	// unused svc to show it isn't populated within a different namespace.
	service3 := testutil.FakeService(namespace2, "unused", "1.2.3.4", "TCP", 8084)

	cacheReader.AddHomeEntry(metav1.NamespaceDefault, apimgmt.Services, service1)
	cacheReader.AddHomeEntry(namespace, apimgmt.Services, service2)
	cacheReader.AddHomeEntry(namespace2, apimgmt.Services, service3)

	envVarName1 := "k1"
	envVarValue1 := "v1"

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "test-pod-name",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Env: []corev1.EnvVar{
						{Name: envVarName1, Value: envVarValue1},
					},
				},
			},
		},
	}

	remoteSvc := service2.DeepCopy()
	remoteSvc.Namespace = "namespace-natted"
	remoteSvc.Spec.ClusterIP = "4.3.2.1" // change clusterIP to remote service, this is the IP that we want in remote pod env vars
	cacheReader.AddForeignEntry(remoteSvc.Namespace, apimgmt.Services, remoteSvc)
	envs := envvars.FromServices([]*corev1.Service{remoteSvc})

	testCases := []struct {
		name               string          // the name of the test case
		enableServiceLinks *bool           // enabling service links
		expectedEnvs       []corev1.EnvVar // a set of expected environment vars
	}{
		{
			name:               "ServiceLinks disabled",
			enableServiceLinks: pointer.BoolPtr(false),
			expectedEnvs: []corev1.EnvVar{
				{Name: envVarName1, Value: envVarValue1},
			},
		},
		{
			name:               "ServiceLinks enabled",
			enableServiceLinks: pointer.BoolPtr(true),
			expectedEnvs: append([]corev1.EnvVar{
				{Name: envVarName1, Value: envVarValue1},
			}, envs...),
		},
	}

	for _, tc := range testCases {
		pod.Spec.EnableServiceLinks = tc.enableServiceLinks

		resPod, err := TranslateServiceEnvVariables(pod, namespace, "namespace-natted", cacheReader)
		assert.NilError(t, err, "[%s]", tc.name)
		assert.Check(t, is.DeepEqual(resPod.Spec.Containers[0].Env, tc.expectedEnvs, sortOpt))
	}

}
