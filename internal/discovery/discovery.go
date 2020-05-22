package discovery

import (
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
)

var Namespace string = "default"
var Log = ctrl.Log.WithName("discovery")

// Read ConfigMap and start register and resolver goroutines
func StartDiscovery(namespace string) {
	Namespace = namespace
	SetupConfigmap()
	dc := GetDiscoveryConfig()

	var txt []string
	if dc.EnableAdvertisement {
		txtString, err := dc.TxtData.Encode()
		if err != nil {
			Log.Error(err, err.Error())
			os.Exit(1)
		}
		txt = []string{txtString}

		Log.Info("Starting service advertisement")
		go Register(dc.Name, dc.Service, dc.Domain, dc.Port, txt)
	}

	if dc.EnableDiscovery {
		Log.Info("Starting service discovery")
		go StartResolver(dc.Service, dc.Domain, dc.WaitTime, dc.UpdateTime)
	}
}
