package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"time"
)

type peeringProcessMonitoring struct {
	peeringProcessTime       *prometheus.HistogramVec
	peeringEvents            *prometheus.GaugeVec
	startProcessingTime      time.Time
	initMap                  map[string]bool
	consistencyStartEventMap map[string]bool
	consistencyEndEventMap   map[string]bool
}

func (mon *peeringProcessMonitoring) init(useKubebuilder bool) error {
	mon.peeringProcessTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "liqo_peering_process_execution_time",
			Help:    "The elapsed time (ms) in processing of every liqo component involved in the peering process",
			Buckets: prometheus.LinearBuckets(100, 150, 20),
		},
		[]string{"liqo_component"},
	)

	mon.peeringEvents = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "liqo_peering_event",
			Help: "Main events occurring in liqo components during the peering process",
		},
		[]string{"liqo_component", "event", "status"})

	// this map keeps track if an event has been already triggered or it's the first time.
	// NOTE: it will be removed in future implementations
	mon.initMap = createInitMap()

	// this map prevents a component to expose a metric related to the "begin" of an event more than ones, unless the related "end" event has been exposed
	// start events are usually called within Reconcile functions, this map prevents to have multiple start event and only one end event
	mon.consistencyStartEventMap = createConsistencyEventMap(true)
	mon.consistencyEndEventMap = createConsistencyEventMap(false)

	if useKubebuilder {
		if err := metrics.Registry.Register(mon.peeringProcessTime); err != nil {
			return err
		}
		if err := metrics.Registry.Register(mon.peeringEvents); err != nil {
			return err
		}
	} else {
		if err := prometheus.Register(mon.peeringProcessTime); err != nil {
			return err
		}
		if err := prometheus.Register(mon.peeringEvents); err != nil {
			return err
		}
	}

	return nil
}

func (mon *peeringProcessMonitoring) StartComp(component LiqoComponent) {
	panic("Not implemented")
}

func (mon *peeringProcessMonitoring) Start() {
	mon.startProcessingTime = time.Now()
}

func (mon *peeringProcessMonitoring) Complete(component LiqoComponent) {
	processingTimeMS := (time.Now().UnixNano() - mon.startProcessingTime.UnixNano()) / 1000000
	mon.peeringProcessTime.WithLabelValues(component.String()).Observe(float64(processingTimeMS))
}

func (mon *peeringProcessMonitoring) EventRegister(component LiqoComponent, event EventType, status EventStatus) {
	mapKey := component.String() + event.String()

	if status == End {
		if mon.consistencyEndEventMap[mapKey] {
			mon.peeringEvents.WithLabelValues(component.String(), event.String(), status.String()).Inc()

			mon.consistencyStartEventMap[mapKey] = true
			mon.consistencyEndEventMap[mapKey] = false
		}
	} else {
		if mon.consistencyStartEventMap[mapKey] {
			mon.peeringEvents.WithLabelValues(component.String(), event.String(), status.String()).Inc()

			if mon.initMap[mapKey] {
				mon.peeringEvents.WithLabelValues(component.String(), event.String(), End.String()).Set(0.0)
				mon.initMap[mapKey] = false
			}
			mon.consistencyStartEventMap[mapKey] = false
			mon.consistencyEndEventMap[mapKey] = true
		}
	}
}
