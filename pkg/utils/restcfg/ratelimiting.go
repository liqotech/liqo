// Copyright 2019-2025 The Liqo Authors
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
	"time"

	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
)

const (
	// DefaultClientTimeout -> The default timeout value assigned to client-go clients. 0 means no timeout.
	DefaultClientTimeout = time.Duration(0)
	// DefaultQPS -> The default QPS value assigned to client-go clients.
	DefaultQPS = uint(100)
	// DefaultBurst -> The default burst value assigned to client-go clients.
	DefaultBurst = uint(100)
)

var (
	timeout = DefaultClientTimeout
	qps     = DefaultQPS
	burst   = DefaultBurst
)

// InitFlags initializes the flags to configure the rate limiter parameters.
func InitFlags(flagset *pflag.FlagSet) {
	if flagset == nil {
		flagset = pflag.CommandLine
	}

	flagset.DurationVar(&timeout, "client-timeout", DefaultClientTimeout,
		"The maximum length of time to wait before giving up on a server request. A value of zero means no timeout.")
	flagset.UintVar(&qps, "client-qps", DefaultQPS, "The maximum number of queries per second performed towards the API server.")
	flagset.UintVar(&burst, "client-max-burst", DefaultBurst, "The maximum burst of requests in excess of the rate limit towards the API server.")
}

// SetRateLimiter configures the rate limiting parameters of the given rest configuration
// to the values obtained from the command line parameters.
func SetRateLimiter(cfg *rest.Config) *rest.Config {
	return SetRateLimiterWithCustomParameters(cfg, timeout, float32(qps), int(burst))
}

// SetRateLimiterWithCustomParameters configures the rate limiting parameters of the given rest configuration.
func SetRateLimiterWithCustomParameters(cfg *rest.Config, timeout time.Duration, qps float32, burst int) *rest.Config {
	cfg.Timeout = timeout
	cfg.QPS = qps
	cfg.Burst = burst
	return cfg
}
