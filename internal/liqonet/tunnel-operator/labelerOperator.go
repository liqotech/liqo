// Copyright 2019-2022 The Liqo Authors
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

package tunneloperator

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqoutils "github.com/liqotech/liqo/pkg/liqonet/utils"
)

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=core,resources=services,verbs=list;watch;update

const (
	// These labels are the ones set during the deployment of liqo using the helm chart.
	// Any change to those labels on the helm chart has also to be reflected here.
	podComponentLabelKey   = "app.kubernetes.io/component"
	podComponentLabelValue = "networking"
	podNameLabelKey        = "app.kubernetes.io/name"
	podNameLabelValue      = "gateway"
	gatewayLabelKey        = "net.liqo.io/gateway"
	gatewayStatusActive    = "active"
	gatewayStatusStandby   = "standby"
	serviceAnnotationKey   = "net.liqo.io/gatewayNodeIP"
)

var (
	// LabelSelector instructs the informer to only cache the pod objects that satisfy the selector.
	// Only the pod objects with the right labels will be reconciled.
	LabelSelector = cache.SelectorsByObject{
		&corev1.Pod{}: {
			Label: labels.SelectorFromSet(labels.Set{
				podComponentLabelKey: podComponentLabelValue,
				podNameLabelKey:      podNameLabelValue,
			}),
		},
	}
)

// LabelerController reconciles pods objects, in our case the tunnel operator pods.
type LabelerController struct {
	client.Client
	PodIP string
}

// NewLabelerController  returns a new controller ready to be setup and started with the controller manager.
func NewLabelerController(podIP string, cl client.Client) *LabelerController {
	return &LabelerController{
		Client: cl,
		PodIP:  podIP,
	}
}

// Reconcile for a given pod, replica of the current operator, it checks if it is the current pod
// meaning the pod where this code is running. If it is our pod, it checks that it is labels as the
// active replica of the gateway. It ensures that the label "net.liqo.io/gateway=active" is present.
// If the pod is not the current one, we make sure that the pod has the label "net.liqo.io/gateway=standby".
func (lbc *LabelerController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pod := new(corev1.Pod)
	err := lbc.Get(ctx, req.NamespacedName, pod)
	if err != nil {
		klog.Errorf("an error occurred while getting pod {%s}: %v", req.NamespacedName, err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// If it is our pod/current pod then ensure that the labels values is set to "active".
	if lbc.PodIP == pod.Status.PodIP {
		if liqoutils.AddLabelToObj(pod, gatewayLabelKey, gatewayStatusActive) {
			if err := lbc.Update(ctx, pod); err != nil {
				klog.Errorf("an error occurred while updating value of label {%s} to {%s} for pod {%s}: %v",
					gatewayLabelKey, gatewayStatusActive, req.String(), err)
				return ctrl.Result{}, err
			}
			klog.Infof("successfully updated label {%s: %s} for pod {%s}",
				gatewayLabelKey, gatewayStatusActive, req.String())
		}
		if err := lbc.annotateGatewayService(ctx); err != nil {
			// Do not log here, already done in annotateGatewayService.
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	// Make sure that the other replicas has the label set to "standby".
	if val := liqoutils.GetLabelValueFromObj(pod, gatewayLabelKey); val == gatewayStatusActive {
		if liqoutils.AddLabelToObj(pod, gatewayLabelKey, gatewayStatusStandby) {
			if err := lbc.Update(ctx, pod); err != nil {
				klog.Errorf("an error occurred while updating value of label {%s} to {%s} for pod {%s}: %v",
					gatewayLabelKey, gatewayStatusStandby, req.String(), err)
				return ctrl.Result{}, err
			}
			klog.Infof("successfully updated label {%s: %s} for pod {%s}",
				gatewayLabelKey, gatewayStatusStandby, req.String())
		}
	}
	return ctrl.Result{}, nil
}

func (lbc *LabelerController) annotateGatewayService(ctx context.Context) error {
	const expectedNumOfServices = 1
	svcList := new(corev1.ServiceList)
	labelsSelector := client.MatchingLabels{
		podComponentLabelKey: podComponentLabelValue,
		podNameLabelKey:      podNameLabelValue,
	}
	err := lbc.List(ctx, svcList, labelsSelector)
	if err != nil {
		return err
	}
	if len(svcList.Items) != expectedNumOfServices {
		klog.Errorf("an error occurred while getting gateway service: expected number of services for the gateway is {%d}, "+
			"instead we found {%d}", expectedNumOfServices, len(svcList.Items))
		return fmt.Errorf("expected number of services for the gateway is {%d}, instead we found {%d}",
			expectedNumOfServices, len(svcList.Items))
	}
	// We come here only if one service has been found.
	svc := &svcList.Items[0]
	if liqoutils.AddAnnotationToObj(svc, serviceAnnotationKey, lbc.PodIP) {
		if err := lbc.Update(ctx, svc); err != nil {
			klog.Errorf("an error occurred while annotating gateway service {%s/%s}: %v",
				svc.Namespace, svc.Name, serviceAnnotationKey, err)
			return err
		}
		klog.Infof("successfully annotated gateway service {%s/%s} with annotation {%s: %s}",
			svc.Namespace, svc.Name, serviceAnnotationKey, lbc.PodIP)
	}
	return nil
}

// SetupWithManager used to set up the controller with a given manager.
func (lbc *LabelerController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&corev1.Pod{}).
		Complete(lbc)
}
