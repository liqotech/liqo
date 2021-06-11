package routeoperator

import (
	"context"
	"net"

	corev1 "k8s.io/api/core/v1"
	k8sApiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	liqoerrors "github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
)

var (
	// This labels are the ones set during the deployment of liqo using the helm chart.
	// Any change to those labels on the helm chart has also to be reflected here.
	podInstanceLabelKey   = "app.kubernetes.io/instance"
	podInstanceLabelValue = "liqo-route"
	podNameLabelKey       = "app.kubernetes.io/name"
	podNameLabelValue     = "route"
	// vxlanMACAddressKey annotation key the mac address of vxlan interface.
	vxlanMACAddressKey = "net.liqo.io/vxlan.mac.address"
	// PodLabelSelector label selector used to track only the route pods.
	PodLabelSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      podInstanceLabelKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{podInstanceLabelValue},
			},
			{
				Key:      podNameLabelKey,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{podNameLabelValue},
			},
		},
	}
)

// OverlayController reconciles pods objects, in our case the route operators pods.
type OverlayController struct {
	vxlanDev overlay.VxlanDevice
	client.Client
	podName     string
	podIP       string
	podSelector labels.Selector
	vxlanPeers  map[string]*overlay.Neighbor
}

// Reconcile for a given pod it checks if it is our pod or not. If it is our pod than annotates
// it with mac address of the current vxlan device. If it is a pod running in a different node
// then based on the type of event:
// event.Create/Update it adds the peer to the vxlan overlay network if it does not exist.
// event.Delete it removes the peer from the vxlan overlay network if it does exist.
func (ovc *OverlayController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var pod corev1.Pod
	var err error
	err = ovc.Get(ctx, req.NamespacedName, &pod)
	if err != nil && !k8sApiErrors.IsNotFound(err) {
		klog.Errorf("an error occurred while getting pod %s: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}
	if k8sApiErrors.IsNotFound(err) {
		// Remove the peer.
		deleted, err := ovc.delPeer(req)
		if err != nil {
			return ctrl.Result{}, err
		}
		if deleted {
			klog.Infof("successfully removed peer %s from vxlan overlay network", req.String())
		}
		return ctrl.Result{}, nil
	}
	// If it is our pod than add the mac address annotation.
	if ovc.podIP == pod.Status.PodIP {
		if ovc.addAnnotation(&pod, vxlanMACAddressKey, ovc.vxlanDev.Link.HardwareAddr.String()) {
			if err := ovc.Update(ctx, &pod); err != nil {
				klog.Errorf("an error occurred while adding mac address annotation to pod %s: %v", req.String(), err)
				return ctrl.Result{}, err
			}
			klog.Infof("successfully annotated pod %s with mac address %s", req.String(), ovc.vxlanDev.Link.HardwareAddr.String())
		}
		return ctrl.Result{}, nil
	}

	// If it is not our pod, then add peer to the vxlan network.
	added, err := ovc.addPeer(req, &pod)
	if err != nil {
		klog.Errorf("an error occurred while adding peer %s with IP address %s and MAC address %s to the vxlan overlay network: %v",
			req.String(), pod.Status.PodIP, ovc.getAnnotationValue(&pod, vxlanMACAddressKey), err)
		return ctrl.Result{}, err
	}
	if added {
		klog.Errorf("successfully added peer %s with IP address %s and MAC address %s to the vxlan overlay network",
			req.String(), pod.Status.PodIP, ovc.getAnnotationValue(&pod, vxlanMACAddressKey))
	}
	return ctrl.Result{}, nil
}

// NewOverlayController returns a new controller ready to be setup and started with the controller manager.
func NewOverlayController(podName, podIP string, podSelector *metav1.LabelSelector,
	vxlanDevice overlay.VxlanDevice, cl client.Client) (*OverlayController, error) {
	selector, err := metav1.LabelSelectorAsSelector(podSelector)
	if err != nil {
		return nil, err
	}
	if vxlanDevice.Link == nil {
		return nil, &liqoerrors.WrongParameter{
			Reason:    liqoerrors.NotNil,
			Parameter: "vxlanDevice.Link",
		}
	}
	return &OverlayController{
		vxlanDev:    vxlanDevice,
		Client:      cl,
		podName:     podName,
		podIP:       podIP,
		podSelector: selector,
		vxlanPeers:  make(map[string]*overlay.Neighbor),
	}, nil
}

// addPeer for a given pod it adds the fdb entry for the current vxlan device.
// It return true when the entry does not exist and is added, false if the entry does already exist,
// and error if something goes wrong.
func (ovc *OverlayController) addPeer(req ctrl.Request, pod *corev1.Pod) (bool, error) {
	peerIP := pod.Status.PodIP
	peerMAC := pod.GetAnnotations()[vxlanMACAddressKey]
	ip := net.ParseIP(peerIP)
	if ip == nil {
		return false, &liqoerrors.ParseIPError{IPToBeParsed: peerIP}
	}
	mac, err := net.ParseMAC(peerMAC)
	if err != nil {
		return false, err
	}
	peer := overlay.Neighbor{
		MAC: mac,
		IP:  ip,
	}

	added, err := ovc.vxlanDev.AddFDB(peer)
	if err != nil {
		return added, err
	}
	ovc.vxlanPeers[req.String()] = &peer
	return added, nil
}

// delPeer for a given pod it removes the fdb entry for the current vxlan device.
// It return true when the entry exists and is removed, false if the entry does not exist,
// and error if something goes wrong.
func (ovc *OverlayController) delPeer(req ctrl.Request) (bool, error) {
	peer, ok := ovc.vxlanPeers[req.String()]
	if !ok {
		return false, nil
	}
	deleted, err := ovc.vxlanDev.DelFDB(*peer)
	if err != nil {
		return deleted, err
	}
	delete(ovc.vxlanPeers, req.String())
	return deleted, nil
}

// addAnnotation for a given object it adds the annotation with the given key and value.
// It return a bool which is true when the annotations has been added or false if the
// annotation is already present.
func (ovc *OverlayController) addAnnotation(obj client.Object, aKey, aValue string) bool {
	annotations := obj.GetAnnotations()
	oldAnnValue, ok := annotations[aKey]
	// If the annotations does not exist or is outdated then set it.
	if !ok || oldAnnValue != aValue {
		annotations[aKey] = aValue
		obj.SetAnnotations(annotations)
		return true
	}
	return false
}

// getAnnotationValue all objects passed to this function has the annotations set.
// The podFilter functions makes sure that we reconcile only objects with the annotation set.
func (ovc *OverlayController) getAnnotationValue(obj client.Object, akey string) string {
	return obj.GetAnnotations()[akey]
}

// podFilter used to filter out all the pods that are not instances of the route operator
// daemon set. It checks that pods are route operator instances, and has the vxlanMACAddressKey
// annotation set or that the current pod we are considering is our same pod. In this case
// we add the vxlanMACAddressKey annotation in order for the other nodes to add as to the overlay network.
func (ovc *OverlayController) podFilter(obj client.Object) bool {
	// Check if the object is a pod.
	p, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Infof("object %s is not of type corev1.Pod", obj.GetName())
		return false
	}
	// Filter by labels.
	if match := ovc.podSelector.Matches(labels.Set(obj.GetLabels())); !match {
		return false
	}
	// If it is our pod then process it.
	if ovc.podIP == p.Status.PodIP {
		return true
	}
	// If it is not our pod then check if the vxlan mac address has been set.
	annotations := obj.GetAnnotations()
	if _, ok := annotations[vxlanMACAddressKey]; ok {
		return true
	}
	klog.V(4).Infof("route-operator pod {%s} in namespace {%s} running on node {%s} does not have {%s} annotation set",
		p.Name, p.Namespace, p.Spec.NodeName, vxlanMACAddressKey)
	return false
}

// SetupWithManager used to set up the controller with a given manager.
func (ovc *OverlayController) SetupWithManager(mgr ctrl.Manager) error {
	p := predicate.NewPredicateFuncs(ovc.podFilter)
	return ctrl.NewControllerManagedBy(mgr).For(&corev1.Pod{}).WithEventFilter(p).
		Complete(ovc)
}
