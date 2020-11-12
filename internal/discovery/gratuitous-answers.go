package discovery

import "time"

func (discovery *DiscoveryCtrl) StartGratuitousAnswers() {
	for range time.NewTicker(12 * time.Second).C {
		if discovery.Config.EnableAdvertisement {
			discovery.sendAnswer()
		}
	}
}

func (discovery *DiscoveryCtrl) sendAnswer() {
	discovery.serverMux.Lock()
	defer discovery.serverMux.Unlock()
	if discovery.mdnsServer != nil {
		discovery.mdnsServer.SendMulticast()
	}
	if discovery.mdnsServerAuth != nil {
		discovery.mdnsServerAuth.SendMulticast()
	}
}
