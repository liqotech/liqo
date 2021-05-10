package discovery

import "time"

func (discovery *Controller) startGratuitousAnswers() {
	for range time.NewTicker(12 * time.Second).C {
		if discovery.Config.EnableAdvertisement {
			discovery.sendAnswer()
		}
	}
}

func (discovery *Controller) sendAnswer() {
	discovery.serverMux.Lock()
	defer discovery.serverMux.Unlock()
	if discovery.mdnsServerAuth != nil {
		discovery.mdnsServerAuth.SendMulticast()
	}
}
