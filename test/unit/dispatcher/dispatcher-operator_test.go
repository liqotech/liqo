package dispatcher

import (
	"github.com/liqoTech/liqo/internal/dispatcher"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

func setupDispatcherOperator() error {
	var err error
	localDynClient := dynamic.NewForConfigOrDie(k8sManagerLocal.GetConfig())
	dOperator = &dispatcher.DispatcherReconciler{
		Scheme:           k8sManagerLocal.GetScheme(),
		RemoteDynClients: make(map[string]dynamic.Interface),
		LocalDynClient:   localDynClient,
		RunningWatchers:  make(map[string]chan bool),
	}
	err = dOperator.SetupWithManager(k8sManagerLocal)
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}
	return nil
}
