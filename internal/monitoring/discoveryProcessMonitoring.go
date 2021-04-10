package monitoring

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

type discoveryProcessMonitoring struct {
	discoveryProcessTime     *prometheus.HistogramVec
	discoveryEvents          *prometheus.GaugeVec
	startProcessingTime      map[string]time.Time
	initMap                  map[string]bool
	consistencyStartEventMap map[string]bool
	consistencyEndEventMap   map[string]bool
	counter map[string]int
	genMutex sync.Mutex
}

func (mon *discoveryProcessMonitoring) init(useKubebuilder bool) error {
	mon.discoveryProcessTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "liqo_discovery_process_execution_time",
			Help:    "The elapsed time (ms) in processing of every liqo component involved in the discovery process",
			Buckets: prometheus.LinearBuckets(100, 150, 20),
		},
		[]string{"liqo_component"},
	)

	mon.discoveryEvents = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "liqo_discovery_event",
			Help: "Main events occurring in liqo components during the discovery process",
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
		if err := metrics.Registry.Register(mon.discoveryProcessTime); err != nil {
			return err
		}
		if err := metrics.Registry.Register(mon.discoveryEvents); err != nil {
			return err
		}
	} else {
		if err := prometheus.Register(mon.discoveryProcessTime); err != nil {
			return err
		}
		if err := prometheus.Register(mon.discoveryEvents); err != nil {
			return err
		}
	}

	return nil
}

func (mon *discoveryProcessMonitoring) StartComp(component LiqoComponent) {
	mon.genMutex.Lock()
	defer mon.genMutex.Unlock()
	if mon.counter == nil || mon.startProcessingTime == nil {
		mon.counter = map[string]int{}
		mon.startProcessingTime = map[string]time.Time{}
	}
	if v, ok := mon.counter[component.String()]; !ok {
		mon.counter[component.String()] = 0
		mon.startProcessingTime[component.String()] = time.Now()
	} else if v == 0 {
		mon.startProcessingTime[component.String()] = time.Now()
	}
	mon.counter[component.String()] += 1
}

func (mon *discoveryProcessMonitoring) Start() {
	mon.genMutex.Lock()
	defer mon.genMutex.Unlock()
	mon.startProcessingTime[""] = time.Now()
}

func (mon *discoveryProcessMonitoring) Complete(component LiqoComponent) {
	mon.genMutex.Lock()
	defer mon.genMutex.Unlock()
	if mon.counter == nil || mon.startProcessingTime == nil {
		mon.counter = map[string]int{}
		mon.startProcessingTime = map[string]time.Time{}
	}
	if v, ok := mon.counter[component.String()]; !ok {
		mon.counter[component.String()] = 0
		mon.startProcessingTime[component.String()] = time.Now()
	} else if v == 0 {
		mon.startProcessingTime[component.String()] = time.Now()
	}
	mon.counter[component.String()] -= 1
	processingTimeMS := (time.Now().UnixNano() - mon.startProcessingTime[component.String()].UnixNano()) / 1000000
	mon.discoveryProcessTime.WithLabelValues(component.String()).Observe(float64(processingTimeMS))
}

func (mon *discoveryProcessMonitoring) EventRegister(component LiqoComponent, event EventType, status EventStatus) {
	mapKey := component.String() + event.String()

	if status == End {
		if mon.consistencyEndEventMap[mapKey] {
			mon.discoveryEvents.WithLabelValues(component.String(), event.String(), status.String()).Inc()

			mon.consistencyStartEventMap[mapKey] = true
			mon.consistencyEndEventMap[mapKey] = false
		}
	} else {
		if mon.consistencyStartEventMap[mapKey] {
			mon.discoveryEvents.WithLabelValues(component.String(), event.String(), status.String()).Inc()

			if mon.initMap[mapKey] {
				mon.discoveryEvents.WithLabelValues(component.String(), event.String(), End.String()).Set(0.0)
				mon.initMap[mapKey] = false
			}
			mon.consistencyStartEventMap[mapKey] = false
			mon.consistencyEndEventMap[mapKey] = true
		}
	}
}
