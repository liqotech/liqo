package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog"
	"net/http"
)

// TODO: use kubebuilder metric endpoint https://book.kubebuilder.io/reference/metrics.html#publishing-additional-metrics

var (
	peeringMonitoring   peeringProcessMonitoring
	discoveryMonitoring discoveryProcessMonitoring
)

func InitWithKubebuilderEndpoint() error {
	if err := peeringMonitoring.init(true); err != nil {
		return err
	}
	if err := discoveryMonitoring.init(true); err != nil {
		return err
	}

	return nil
}

func Init(metricsPort string) error {
	if err := peeringMonitoring.init(false); err != nil {
		return err
	}
	if err := discoveryMonitoring.init(false); err != nil {
		return err
	}

	go func() {
		http.Handle("/metrics", promhttp.HandlerFor(
			prometheus.DefaultGatherer,
			promhttp.HandlerOpts{
				// Opt into OpenMetrics to support exemplars.
				EnableOpenMetrics: true,
			},
		))
		// There may be multiple calls to the init function, the first one will succeed in the port binding
		// the following ones will fail. One HTTP handler will always be available
		err := http.ListenAndServe(metricsPort, nil)
		klog.Warning(err)
	}()

	return nil
}

func Serve(port string) {
	go func() {
		// There may be multiple calls to the init function, the first one will succeed in the port binding
		// the following ones will fail. One HTTP handler will always be available
		err := http.ListenAndServe(port, nil)
		klog.Warning(err)
	}()
}

func GetDiscoveryProcessMonitoring() ProcessMonitoring {
	return &discoveryMonitoring
}

func GetPeeringProcessMonitoring() ProcessMonitoring {
	return &peeringMonitoring
}
