package advertisement_operator

import (
	"context"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	pkg "github.com/liqotech/liqo/pkg/advertisement-operator"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
	"time"
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

func createFakePod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "image1",
				},
			},
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

	deploy := pkg.CreateVkDeployment(adv, vkName, vkNamespace, vkImage, initVkImage, nodeName, homeClusterId, "127.0.0.1", "6443")

	assert.Equal(t, vkName, deploy.Name)
	assert.Equal(t, vkNamespace, deploy.Namespace)
	assert.Equal(t, pkg.GetOwnerReference(adv), deploy.OwnerReferences)
	assert.Equal(t, adv.Spec.ClusterId, deploy.Spec.Template.Labels["cluster"])
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

func TestCreateOrUpdate(t *testing.T) {
	c, _ := createFakeKubebuilderClient()

	testPod(t, c)
	testAdvertisement(t, c)
}

func testPod(t *testing.T, c client.Client) {
	name, ns := "pod", "fakens"
	nsObject := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	key := types.NamespacedName{
		Name:      name,
		Namespace: ns,
	}
	err := c.Create(context.Background(), nsObject)
	assert.Nil(t, err)
	pod := createFakePod(name, ns)

	// test pod creation
	err = pkg.CreateOrUpdate(c, context.Background(), pod)
	assert.Nil(t, err)

	// creation requires some time to be effective
	// wait 1 second to assure the resource is retrieved by Get
	time.Sleep(1 * time.Second)

	// test pod update
	pod.Spec.Containers[0].Image = "image2"
	err = pkg.CreateOrUpdate(c, context.Background(), pod)
	assert.Nil(t, err)

	time.Sleep(1 * time.Second)

	var pod2 corev1.Pod
	err = c.Get(context.Background(), key, &pod2)
	assert.Nil(t, err)
	assert.Equal(t, pod.Spec.Containers[0].Image, pod2.Spec.Containers[0].Image)
}

func testAdvertisement(t *testing.T, c client.Client) {
	name, ns := "advertisement-cluster1", "fakens"
	adv := createFakeAdv(name, ns)

	// test adv creation
	err := pkg.CreateOrUpdate(c, context.Background(), adv)
	assert.Nil(t, err)

	//TODO: update test
	// adv creation is not effective and Get returns an error
}
