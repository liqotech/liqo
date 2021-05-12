// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package module

import (
	"context"
	"testing"
	"time"

	"golang.org/x/time/rate"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/workqueue"

	testutil "github.com/liqotech/liqo/pkg/virtualKubelet/test/util"
)

type TestController struct {
	*PodController
	mock   *mockProviderAsync
	client *fake.Clientset
}

func newTestController() *TestController {
	fk8s := fake.NewSimpleClientset()

	rm := testutil.FakeResourceManager()
	p := newMockProvider()
	iFactory := kubeinformers.NewSharedInformerFactoryWithOptions(fk8s, 10*time.Minute)
	podController, err := NewPodController(PodControllerConfig{
		PodClient:         fk8s.CoreV1(),
		PodInformer:       iFactory.Core().V1().Pods(),
		EventRecorder:     testutil.FakeEventRecorder(5),
		Provider:          p,
		ConfigMapInformer: iFactory.Core().V1().ConfigMaps(),
		SecretInformer:    iFactory.Core().V1().Secrets(),
		ServiceInformer:   iFactory.Core().V1().Services(),
		SyncPodsFromKubernetesRateLimiter: workqueue.NewMaxOfRateLimiter(
			// The default upper bound is 1000 seconds. Let's not use that.
			workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 10*time.Millisecond),
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		),
		SyncPodStatusFromProviderRateLimiter: workqueue.NewMaxOfRateLimiter(
			// The default upper bound is 1000 seconds. Let's not use that.
			workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 10*time.Millisecond),
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		),
		DeletePodsFromKubernetesRateLimiter: workqueue.NewMaxOfRateLimiter(
			// The default upper bound is 1000 seconds. Let's not use that.
			workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 10*time.Millisecond),
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		),
	})

	if err != nil {
		panic(err)
	}
	// Override the resource manager in the contructor with our own.
	podController.resourceManager = rm

	return &TestController{
		PodController: podController,
		mock:          p,
		client:        fk8s,
	}
}

// Run starts the informer and runs the pod controller
func (tc *TestController) Run(ctx context.Context, n int) error {
	go tc.podsInformer.Informer().Run(ctx.Done())
	return tc.PodController.Run(ctx, n)
}

func TestPodsEqual(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.15.12-perl",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 443,
							Protocol:      "tcp",
						},
					},
				},
			},
		},
	}

	p2 := p1.DeepCopy()

	assert.Assert(t, podsEqual(p1, p2))
}

func TestPodsDifferent(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: newPodSpec(),
	}

	p2 := p1.DeepCopy()
	p2.Spec.Containers[0].Image = "nginx:1.15.12-perl"

	assert.Assert(t, !podsEqual(p1, p2))
}

func TestPodsDifferentIgnoreValue(t *testing.T) {
	p1 := &corev1.Pod{
		Spec: newPodSpec(),
	}

	p2 := p1.DeepCopy()
	p2.Status.Phase = corev1.PodFailed

	assert.Assert(t, podsEqual(p1, p2))
}

func TestPodCreateNewPod(t *testing.T) {
	svr := newTestController()

	pod := &corev1.Pod{}
	pod.ObjectMeta.Namespace = "default" //nolint:goconst
	pod.ObjectMeta.Name = "nginx"        //nolint:goconst
	pod.Spec = newPodSpec()

	err := svr.createOrUpdatePod(context.Background(), pod.DeepCopy())

	assert.Check(t, is.Nil(err))
	// createOrUpdate called CreatePod but did not call UpdatePod because the pod did not exist
	assert.Check(t, is.Equal(svr.mock.creates.read(), 1))
	assert.Check(t, is.Equal(svr.mock.updates.read(), 0))
}

func TestPodUpdateExisting(t *testing.T) {
	svr := newTestController()

	pod := &corev1.Pod{}
	pod.ObjectMeta.Namespace = "default"
	pod.ObjectMeta.Name = "nginx"
	pod.Spec = newPodSpec()

	err := svr.createOrUpdatePod(context.Background(), pod.DeepCopy())
	assert.Check(t, is.Nil(err))
	assert.Check(t, is.Equal(svr.mock.creates.read(), 1))
	assert.Check(t, is.Equal(svr.mock.updates.read(), 0))

	pod2 := pod.DeepCopy()
	pod2.Spec.Containers[0].Image = "nginx:1.15.12-perl"

	err = svr.createOrUpdatePod(context.Background(), pod2.DeepCopy())
	assert.Check(t, is.Nil(err))

	// createOrUpdate didn't call CreatePod but did call UpdatePod because the spec changed
	assert.Check(t, is.Equal(svr.mock.creates.read(), 1))
	assert.Check(t, is.Equal(svr.mock.updates.read(), 1))
}

func TestPodNoSpecChange(t *testing.T) {
	svr := newTestController()

	pod := &corev1.Pod{}
	pod.ObjectMeta.Namespace = "default"
	pod.ObjectMeta.Name = "nginx"
	pod.Spec = newPodSpec()

	err := svr.createOrUpdatePod(context.Background(), pod.DeepCopy())
	assert.Check(t, is.Nil(err))
	assert.Check(t, is.Equal(svr.mock.creates.read(), 1))
	assert.Check(t, is.Equal(svr.mock.updates.read(), 0))

	err = svr.createOrUpdatePod(context.Background(), pod.DeepCopy())
	assert.Check(t, is.Nil(err))

	// createOrUpdate didn't call CreatePod or UpdatePod, spec didn't change
	assert.Check(t, is.Equal(svr.mock.creates.read(), 1))
	assert.Check(t, is.Equal(svr.mock.updates.read(), 0))
}

func newPodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "nginx",
				Image: "nginx:1.15.12",
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: 443,
						Protocol:      "tcp",
					},
				},
			},
		},
	}
}
