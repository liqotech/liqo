package common

import (
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

// GetLiqoctlRestConfOrDie gets a valid REST config and set a default value for the RateLimiters. It dies otherwise.
func GetLiqoctlRestConfOrDie() *rest.Config {
	return restcfg.SetRateLimiter(config.GetConfigOrDie())
}
