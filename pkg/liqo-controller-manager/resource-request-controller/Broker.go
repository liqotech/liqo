package resourcerequestoperator

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
)

type Broker struct {
	nodeResources map[string]corev1.ResourceList
	nodeInformer  cache.SharedIndexInformer
	podInformer   cache.SharedIndexInformer
	scheme        *runtime.Scheme
	client.Client
	homeClusterID string
	// offerGenerator interfaces.UpdaterInterface
}

func (b *Broker) SetupBroker(clusterID string, clientset kubernetes.Interface, scheme *runtime.Scheme, resyncPeriod time.Duration, k8Client client.Client) {
	b.nodeResources = map[string]corev1.ResourceList{}
	b.Client = k8Client
	b.scheme = scheme
	b.homeClusterID = clusterID
	nodesFactory := informers.NewSharedInformerFactoryWithOptions(clientset, resyncPeriod, informers.WithTweakListOptions(nodeFilter))
	b.nodeInformer = nodesFactory.Core().V1().Nodes().Informer()
	b.nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    b.onNodeAdd,
		DeleteFunc: b.onNodeDelete,
	})
}

func (b *Broker) Start(ctx context.Context, group *sync.WaitGroup) {
	go b.startNodeInformer(ctx, group)
}

func (b *Broker) startNodeInformer(ctx context.Context, group *sync.WaitGroup) {
	group.Add(1)
	defer group.Done()
	b.nodeInformer.Run(ctx.Done())
}

func (b *Broker) ReadResources(clusterID string) corev1.ResourceList {
	return b.nodeResources[clusterID]
}

func (b *Broker) EnqueueForCreationOrUpdate(clusterID string) {
	toOffer := corev1.ResourceList{}
	for key, value := range b.nodeResources {
		// ignore possible offers sent by the cluster itself.
		if key == clusterID {
			continue
		}
		toOffer = value.DeepCopy()
		break
	}
	if len(toOffer) != 0 {
		err := b.generateOffer(clusterID, toOffer)
		if err != nil {
			klog.Error(err)
			return
		}
	}
}

func (b *Broker) generateOffer(clusterID string, toOffer corev1.ResourceList) error {
	list, err := b.getResourceRequest(clusterID)
	if err != nil {
		return err
	} else if len(list.Items) != 1 {
		return fmt.Errorf("ClusterID %s is no more valid. Deleting", clusterID)
	}
	request := list.Items[0]
	offer := &sharingv1alpha1.ResourceOffer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: request.GetNamespace(),
			Name:      offerPrefix + b.homeClusterID,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.Background(), b.Client, offer, func() error {
		if offer.Labels != nil {
			offer.Labels[discovery.ClusterIDLabel] = request.Spec.ClusterIdentity.ClusterID
			offer.Labels[crdreplicator.LocalLabelSelector] = "true"
			offer.Labels[crdreplicator.DestinationLabel] = request.Spec.ClusterIdentity.ClusterID
		} else {
			offer.Labels = map[string]string{
				discovery.ClusterIDLabel:         request.Spec.ClusterIdentity.ClusterID,
				crdreplicator.LocalLabelSelector: "true",
				crdreplicator.DestinationLabel:   request.Spec.ClusterIdentity.ClusterID,
			}
		}
		offer.Spec.ClusterId = b.homeClusterID
		offer.Spec.ResourceQuota.Hard = toOffer.DeepCopy()
		return controllerutil.SetControllerReference(&request, offer, b.scheme)
	})

	if err != nil {
		klog.Error(err)
		return err
	}
	klog.Infof("%s -> %s Offer: %s/%s", b.homeClusterID, op, offer.Namespace, offer.Name)
	return nil
}

func (b *Broker) getResourceRequest(clusterID string) (*discoveryv1alpha1.ResourceRequestList, error) {
	resourceRequestList := &discoveryv1alpha1.ResourceRequestList{}
	err := b.Client.List(context.Background(), resourceRequestList, client.MatchingLabels{
		crdreplicator.RemoteLabelSelector: clusterID,
	})
	if err != nil {
		return nil, err
	}
	return resourceRequestList, nil
}

func (b *Broker) RemoveClusterID(clusterID string) {
	delete(b.nodeResources, clusterID)
}

func (b *Broker) onNodeAdd(obj interface{}) {
	node := obj.(*corev1.Node)
	//if utils.IsNodeReady(node) {
	if clusterID, ok := node.GetAnnotations()[consts.RemoteClusterID]; ok {
		klog.V(4).Infof("Created virtual node %s\n", node.Name)
		toAdd, err := b.getClusterOffer(clusterID)
		if err != nil {
			return
		}
		b.nodeResources[clusterID] = toAdd.DeepCopy()
	}
	//}
}

func (b *Broker) onNodeDelete(obj interface{}) {
	node := obj.(*corev1.Node)
	// if utils.IsNodeReady(node) {
	if clusterID, ok := node.GetAnnotations()[consts.RemoteClusterID]; ok {
		klog.V(4).Infof("Deleting virtual node %s\n", node.Name)
		delete(b.nodeResources, clusterID)
	}
	//}
}

func (b *Broker) getClusterOffer(clusterID string) (corev1.ResourceList, error) {
	offerList := &sharingv1alpha1.ResourceOfferList{}
	err := b.Client.List(context.Background(), offerList, client.MatchingLabels{
		crdreplicator.RemoteLabelSelector: clusterID,
	})
	if err != nil {
		return nil, err
	}

	if len(offerList.Items) != 1 {
		return nil, fmt.Errorf("too much offers for cluster %s", clusterID)
	}

	return offerList.Items[0].Spec.ResourceQuota.Hard, nil
}

func nodeFilter(options *metav1.ListOptions) {
	var values []string
	values = append(values, consts.TypeNode)
	req, err := labels.NewRequirement(consts.TypeLabel, selection.Equals, values)
	if err != nil {
		return
	}
	options.LabelSelector = labels.NewSelector().Add(*req).String()
}
