package client

import (
	advtypes "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	"github.com/liqoTech/liqo/pkg/crdClient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"time"
)

//NotifyChannelType identifies a notification channel for a specific event
type NotifyChannelType int

//NotifyChannelType identifiers
const (
	//Notification channel id for the creation of an Advertisement
	ChanAdvNew NotifyChannelType = iota
	//Notification channel id for the acceptance of an Advertisement
	ChanAdvAccepted
	//Notification channel id for the deletion of an Advertisement
	ChanAdvDeleted
	//Notification channel id for the revocation of the 'ACCEPTED' status of an Advertisement
	ChanAdvRevoked
)
const notifyBuffLength = 100

//AdvertisementCache defines a data structure that provides a kubernetes cache for the Advertisement CRD along with
//related status information and a controller.
type AdvertisementCache struct {
	// kubernetes cache for the Advertisement CRD.
	Store cache.Store
	// controller of the cache. Close() this channel to stop it.
	Controller chan struct{}
	// specifies whether the AdvertisementCache is up and running
	Running bool
	// set of the channels used by the AdvertisementCache logic to notify a watched event
	NotifyChannels map[NotifyChannelType]chan string
}

//creates and initializes an AdvertisementCache
func createAdvCache() *AdvertisementCache {
	ac := AdvertisementCache{
		NotifyChannels: make(map[NotifyChannelType]chan string)}
	ac.NotifyChannels[ChanAdvNew] = make(chan string, notifyBuffLength)
	ac.NotifyChannels[ChanAdvAccepted] = make(chan string, notifyBuffLength)
	ac.NotifyChannels[ChanAdvDeleted] = make(chan string, notifyBuffLength)
	ac.NotifyChannels[ChanAdvRevoked] = make(chan string, notifyBuffLength)
	return &ac
}

//StartCache starts a watch (if not already running) on the Advertisement CRD in the cluster,
//storing data in the AdvertisementCache.
func (c *AdvertisementCache) StartCache(client *crdClient.CRDClient) error {
	if c.Running {
		return nil
	}
	if !mockedController {
		ehf := cache.ResourceEventHandlerFuncs{
			AddFunc:    checkNewAdv,
			UpdateFunc: updateAcceptedAdv,
			DeleteFunc: deleteAcceptedAdv,
		}
		lo := metav1.ListOptions{}
		var err error
		c.Store, c.Controller, err = crdClient.WatchResources(client,
			"advertisements", "",
			time.Second*2, ehf, lo)
		if err == nil {
			c.Running = true
		}
		return err
	} else {
		c.Controller = make(chan struct{})
		c.Running = true
		return nil
	}
}

//StopCache stops (if running) the watch on the Advertisement CRD associated with the cache. Moreover, all
//the associated NotifyChannels are closed.
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
		agentCtrl.advCache.NotifyChannels[ChanAdvAccepted] <- newAdv.Name
	} else {
		agentCtrl.advCache.NotifyChannels[ChanAdvNew] <- newAdv.Name
	}
}

// callback function for the Advertisement watch.
func updateAcceptedAdv(oldObj interface{}, newObj interface{}) {
	oldAdv := oldObj.(*advtypes.Advertisement)
	newAdv := newObj.(*advtypes.Advertisement)
	if oldAdv.Status.AdvertisementStatus != "ACCEPTED" && newAdv.Status.AdvertisementStatus == "ACCEPTED" {
		agentCtrl.advCache.NotifyChannels[ChanAdvAccepted] <- newAdv.Name
	} else if oldAdv.Status.AdvertisementStatus == "ACCEPTED" && newAdv.Status.AdvertisementStatus != "ACCEPTED" {
		agentCtrl.advCache.NotifyChannels[ChanAdvRevoked] <- newAdv.Name
	}
}

// callback function for the Advertisement watch.
func deleteAcceptedAdv(obj interface{}) {
	adv := obj.(*advtypes.Advertisement)
	agentCtrl.advCache.NotifyChannels[ChanAdvDeleted] <- adv.Name
}
