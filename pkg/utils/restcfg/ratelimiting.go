// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package restcfg

import (
	"flag"

	"k8s.io/client-go/rest"
)

const (
	// DefaultQPS -> The default QPS value assigned to client-go clients.
	DefaultQPS = uint(100)
	// DefaultBurst -> The default burst value assigned to client-go clients.
	DefaultBurst = uint(100)
)

var (
	qps   = DefaultQPS
	burst = DefaultBurst
)

// InitFlags initializes the flags to configure the rate limiter parameters.
func InitFlags(flagset *flag.FlagSet) {
	if flagset == nil {
		flagset = flag.CommandLine
	}

	flagset.UintVar(&qps, "client-qps", qps, "The maximum number of queries per second performed towards the API server.")
	flagset.UintVar(&burst, "client-max-burst", qps, "The maximum burst of requests in excess of the rate limit towards the API server.")
}

// SetRateLimiter configures the rate limiting parameters of the given rest configuration
// to the values obtained from the command line parameters.
func SetRateLimiter(cfg *rest.Config) *rest.Config {
	return SetRateLimiterWithCustomParameters(cfg, float32(qps), int(burst))
}

// SetRateLimiterWithCustomParameters configures the rate limiting parameters of the given rest configuration.
func SetRateLimiterWithCustomParameters(cfg *rest.Config, qps float32, burst int) *rest.Config {
	cfg.QPS = qps
	cfg.Burst = burst
	return cfg
}
