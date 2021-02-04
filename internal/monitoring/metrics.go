package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

var (
	peeringProcessTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "liqo_peering_process_execution_time",
			Help:    "The elapsed time (ms) in processing of every liqo component involved in the peering process",
			Buckets: prometheus.LinearBuckets(100, 150, 20),
		},
		[]string{"liqo_component"},
	)

	peeringEvents = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "liqo_peering_event",
			Help: "Main events occurring in liqo components during the peering process",
		},
		[]string{"liqo_component", "event", "status"})

	startProcessingTime time.Time

	// this map keeps track if an event has been already triggered or it's the first time.
	// NOTE: it will be removed in future implementations
	initMap = createInitMap()

	// this map prevents a component to expose a metric related to the "begin" of an event more than ones, unless the related "end" event has been exposed
	// start events are usually called within Reconcile functions, this map prevents to have multiple start event and only one end event
	consistencyStartEventMap = createConsistencyEventMap(true)
	consistencyEndEventMap   = createConsistencyEventMap(false)
)

func init() {
	// Register custom metrics with the global prometheus registry
	prometheus.MustRegister(peeringProcessTime)
	prometheus.MustRegister(peeringEvents)

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
		_ = http.ListenAndServe(":8090", nil)
	}()
}

func PeeringProcessExecutionStarted() {
	startProcessingTime = time.Now()
}

func PeeringProcessExecutionCompleted(component LiqoComponent) {
	processingTimeMS := (time.Now().UnixNano() - startProcessingTime.UnixNano()) / 1000000
	peeringProcessTime.WithLabelValues(component.String()).Observe(float64(processingTimeMS))
}

func PeeringProcessEventRegister(component LiqoComponent, event EventType, status EventStatus) {
	mapKey := component.String() + event.String()

	if status == End {
		if consistencyEndEventMap[mapKey] {
			peeringEvents.WithLabelValues(component.String(), event.String(), status.String()).Inc()

			consistencyStartEventMap[mapKey] = true
			consistencyEndEventMap[mapKey] = false
		}
	} else {
		if consistencyStartEventMap[mapKey] {
			peeringEvents.WithLabelValues(component.String(), event.String(), status.String()).Inc()

			if initMap[mapKey] {
				peeringEvents.WithLabelValues(component.String(), event.String(), End.String()).Set(0.0)
				initMap[mapKey] = false
			}
			consistencyStartEventMap[mapKey] = false
			consistencyEndEventMap[mapKey] = true
		}
	}
}

func createInitMap() map[string]bool {
	retMap := make(map[string]bool)

	for i := LiqoComponent(0); i < lastComponent; i++ {
		for j := EventType(0); j < lastEvent; j++ {
			retMap[i.String()+j.String()] = true
		}
	}

	return retMap
}

func createConsistencyEventMap(initValue bool) map[string]bool {
	retMap := make(map[string]bool)

	for i := LiqoComponent(0); i < lastComponent; i++ {
		for j := EventType(0); j < lastEvent; j++ {
			retMap[i.String()+j.String()] = initValue
		}
	}

	return retMap
}
