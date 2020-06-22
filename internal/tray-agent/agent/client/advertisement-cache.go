package client

import (
	advtypes "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"github.com/liqoTech/liqo/pkg/crdClient/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

//NotifyChannelType identifies a notification channel for a specific event
type NotifyChannelType int

//NotifyChannelType identifiers
const (
	ChanAdvNew NotifyChannelType = iota
	ChanAdvAccepted
	ChanAdvDeleted
	ChanAdvRevoked
)
const notifyBuffLength = 100

type AdvertisementCache struct {
	Store          cache.Store
	Controller     chan struct{}
	Running        bool
	NotifyChannels map[NotifyChannelType]chan string
}

func createAdvCache() *AdvertisementCache {
	ac := AdvertisementCache{
		NotifyChannels: make(map[NotifyChannelType]chan string)}
	ac.NotifyChannels[ChanAdvNew] = make(chan string, notifyBuffLength)
	ac.NotifyChannels[ChanAdvAccepted] = make(chan string, notifyBuffLength)
	ac.NotifyChannels[ChanAdvDeleted] = make(chan string, notifyBuffLength)
	ac.NotifyChannels[ChanAdvRevoked] = make(chan string, notifyBuffLength)
	return &ac
}

//StartCache
func (c *AdvertisementCache) StartCache(client *v1alpha1.CRDClient) {
	if c.Running {
		return
	}
	ehf := cache.ResourceEventHandlerFuncs{
		AddFunc:    checkNewAdv,
		UpdateFunc: updateAcceptedAdv,
		DeleteFunc: deleteAcceptedAdv,
	}
	lo := metav1.ListOptions{}

	c.Store, c.Controller = crdClient.WatchResources(client,
		"advertisements", "",
		0, ehf, lo)
	c.Running = true
}

//StopCache stops the watch on the Advertisement CRD associated with the cache
func (c *AdvertisementCache) StopCache() {
	if c.Running {
		close(c.Controller)
		for _, ch := range c.NotifyChannels {
			close(ch)
		}
		c.Running = false
	}
}

// callback function for the Advertisement watch.
func checkNewAdv(obj interface{}) {
	newAdv := obj.(*advtypes.Advertisement)
	if newAdv.Status.AdvertisementStatus == "ACCEPTED" {
		select {
		case agentCtrl.advCache.NotifyChannels[ChanAdvAccepted] <- newAdv.Name:
		default:
		}
	} else {
		select {
		case agentCtrl.advCache.NotifyChannels[ChanAdvNew] <- newAdv.Name:
		default:
		}
	}
}

// callback function for the Advertisement watch.
func updateAcceptedAdv(oldObj interface{}, newObj interface{}) {
	oldAdv := oldObj.(*advtypes.Advertisement)
	newAdv := newObj.(*advtypes.Advertisement)
	if oldAdv.Status.AdvertisementStatus != "ACCEPTED" && newAdv.Status.AdvertisementStatus == "ACCEPTED" {
		select {
		case agentCtrl.advCache.NotifyChannels[ChanAdvAccepted] <- newAdv.Name:
		default:
		}
	} else if oldAdv.Status.AdvertisementStatus == "ACCEPTED" && newAdv.Status.AdvertisementStatus != "ACCEPTED" {
		select {
		case agentCtrl.advCache.NotifyChannels[ChanAdvRevoked] <- newAdv.Name:
		default:
		}
	}
}

// callback function for the Advertisement watch.
func deleteAcceptedAdv(obj interface{}) {
	adv := obj.(*advtypes.Advertisement)
	select {
	case agentCtrl.advCache.NotifyChannels[ChanAdvDeleted] <- adv.Name:
	default:
	}
}
