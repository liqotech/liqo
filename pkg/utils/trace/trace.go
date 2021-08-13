package trace

import (
	"time"

	"k8s.io/klog/v2"
)

// LongThreshold returns the treshold to show a tracing log, depending on the configured klog level.
func LongThreshold() time.Duration {
	switch {
	case klog.V(5).Enabled():
		return 100 * time.Millisecond
	case klog.V(4).Enabled():
		return 250 * time.Millisecond
	case klog.V(2).Enabled():
		return 500 * time.Millisecond
	default:
		return time.Second
	}
}
