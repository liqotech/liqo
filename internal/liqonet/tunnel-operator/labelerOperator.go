package tunneloperator

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqoutils "github.com/liqotech/liqo/pkg/liqonet/utils"
)

var (
	// This labels are the ones set during the deployment of liqo using the helm chart.
	// Any change to those labels on the helm chart has also to be reflected here.
	podInstanceLabelKey       = "app.kubernetes.io/instance"
	podInstanceLabelValue     = "liqo-gateway"
	podNameLabelKey           = "app.kubernetes.io/name"
	podNameLabelValue         = "gateway"
	serviceSelectorLabelKey   = "net.liqo.io/gatewayPod"
	serviceSelectorLabelValue = "true"
	// LabelSelector instructs the informer to only cache the pod objects that satisfies the selector.
	// Only the pod objects with the right labels will be reconciled.
	LabelSelector = cache.SelectorsByObject{
		&corev1.Pod{}: {
			Label: labels.SelectorFromSet(labels.Set{
				podInstanceLabelKey: podInstanceLabelValue,
				podNameLabelKey:     podNameLabelValue,
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
// meaning the pod where this code is running. If yes, then it adds a the label to the pod if it does
// not have it. If the pod is not the current one the operator makes sure that it does not have the
// label by removing it if present.
func (lbc *LabelerController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pod := new(corev1.Pod)
	err := lbc.Get(ctx, req.NamespacedName, pod)
	if err != nil {
		klog.Errorf("an error occurred while getting pod {%s}: %v", req.NamespacedName, err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// If it is our pod/current pod then add the right label.
	if lbc.PodIP == pod.Status.PodIP {
		if liqoutils.AddLabelToObj(pod, serviceSelectorLabelKey, serviceSelectorLabelValue) {
			if err := lbc.Update(ctx, pod); err != nil {
				klog.Errorf("an error occurred while adding selector label to pod {%s}: %v", req.String(), err)
				return ctrl.Result{}, err
			}
			klog.Infof("successfully added label {%s: %s} to pod {%s}",
				serviceSelectorLabelKey, serviceSelectorLabelValue, req.String())
		}
		return ctrl.Result{}, nil
	}
	// Make sure that the other replicas does not have the selector label.
	if val := liqoutils.GetLabelValueFromObj(pod, serviceSelectorLabelKey); val != "" {
		delete(pod.GetLabels(), serviceSelectorLabelKey)
		if err := lbc.Update(ctx, pod); err != nil {
			klog.Errorf("an error occurred while removing selector label to pod {%s}: %v", req.String(), err)
			return ctrl.Result{}, err
		}
		klog.Infof("successfully removed label {%s: %s} to pod {%s}",
			serviceSelectorLabelKey, serviceSelectorLabelValue, req.String())
	}
	return ctrl.Result{}, nil
}

// SetupWithManager used to set up the controller with a given manager.
func (lbc *LabelerController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&corev1.Pod{}).
		Complete(lbc)
}
