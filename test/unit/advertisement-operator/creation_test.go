package advertisement_operator

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

func createFakeAdv(name, namespace string) *advtypes.Advertisement {
	return &advtypes.Advertisement{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: advtypes.AdvertisementSpec{
			ClusterId: "cluster1",
			KubeConfigRef: corev1.SecretReference{
				Namespace: "fake",
				Name:      "fake-kubeconfig",
			},
			LimitRange: corev1.LimitRangeSpec{Limits: []corev1.LimitRangeItem{}},
			Timestamp:  metav1.NewTime(time.Now()),
			TimeToLive: metav1.NewTime(time.Now().Add(30 * time.Minute)),
		},
	}
}

func createFakeInvalidAdv(name, namespace string, resourceQuota corev1.ResourceQuotaSpec) *advtypes.Advertisement {
	return &advtypes.Advertisement{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: advtypes.AdvertisementSpec{
			ClusterId: "cluster1",
			KubeConfigRef: corev1.SecretReference{
				Namespace: "fake",
				Name:      "fake-kubeconfig",
			},
			ResourceQuota: resourceQuota,
			LimitRange:    corev1.LimitRangeSpec{Limits: []corev1.LimitRangeItem{}},
			Timestamp:     metav1.NewTime(time.Now()),
			TimeToLive:    metav1.NewTime(time.Now().Add(30 * time.Minute)),
		},
	}
}

func createFakeKubebuilderClient() (client.Client, record.EventRecorder) {
	env := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo", "crds")},
	}

	cfg, err := env.Start()
	if err != nil {
		klog.Error(err)
		panic(err)
	}

	if err = advtypes.AddToScheme(scheme.Scheme); err != nil {
		klog.Error(err)
		panic(err)
	}

	manager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		klog.Error(err)
		panic(err)
	}

	cacheStarted := make(chan struct{})
	go func() {
		if err = manager.Start(cacheStarted); err != nil {
			klog.Error(err)
			panic(err)
		}
	}()
	manager.GetCache().WaitForCacheSync(cacheStarted)
	return manager.GetClient(), manager.GetEventRecorderFor("AdvertisementOperator")
}

func TestCreateVkDeployment(t *testing.T) {
	name, ns := "advertisement-cluster1", "fakens"
	adv := createFakeAdv(name, ns)
	vkName := "virtual-kubelet-cluster1"
	nodeName := "liqo-cluster1"
	vkNamespace := "fake"
	vkImage := "liqo/virtual-kubelet"
	initVkImage := "liqo/init-vk"
	homeClusterId := "cluster2"

	deploy, err := forge.CreateVkDeployment(adv, vkName, vkNamespace, vkImage, initVkImage, nodeName, homeClusterId)

	assert.Equal(t, err, nil)
	assert.Equal(t, vkName, deploy.Name)
	assert.Equal(t, vkNamespace, deploy.Namespace)
	assert.Equal(t, adv.Spec.ClusterId, deploy.Spec.Template.Labels["liqo.io/cluster-id"])
	assert.Len(t, deploy.Spec.Template.Spec.Volumes, 2)
	assert.Equal(t, adv.Spec.KubeConfigRef.Name, deploy.Spec.Template.Spec.Volumes[0].VolumeSource.Secret.SecretName)
	assert.Equal(t, initVkImage, deploy.Spec.Template.Spec.InitContainers[0].Image)
	assert.Equal(t, vkImage, deploy.Spec.Template.Spec.Containers[0].Image)
	assert.NotEmpty(t, deploy.Spec.Template.Spec.Containers[0].Args)
	assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].Args, adv.Spec.ClusterId)
	assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].Args, nodeName)
	assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].Args, vkNamespace)
	assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].Args, homeClusterId)
	assert.NotEmpty(t, deploy.Spec.Template.Spec.Containers[0].Command)
	assert.NotEmpty(t, deploy.Spec.Template.Spec.Containers[0].VolumeMounts)
	assert.NotEmpty(t, deploy.Spec.Template.Spec.Containers[0].Env)
	assert.Equal(t, vkName, deploy.Spec.Template.Spec.ServiceAccountName)
	assert.NotEmpty(t, deploy.Spec.Template.Spec.Affinity)
}
