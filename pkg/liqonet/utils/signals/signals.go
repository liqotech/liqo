// Copyright 2019-2023 The Liqo Authors
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

package signals

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var (
	// ShutdownSignals signals used to terminate the programs.
	ShutdownSignals = []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL}
)

// NotifyPosix returns a channel that receives POSIX signals.
// Like os/signal.Notify, but for POSIX signals.
func NotifyPosix(c chan<- os.Signal, signals ...syscall.Signal) {
	osSignals := make([]os.Signal, len(signals))
	for i, sig := range signals {
		osSignals[i] = os.Signal(sig)
	}
	signal.Notify(c, osSignals...)
}

// NotifyContextPosix returns a context that is canceled when one of the given POSIX signals is received.
// Like os/signal.NotifyContext, but for POSIX signals.
func NotifyContextPosix(ctx context.Context, signals ...syscall.Signal) (context.Context, context.CancelFunc) {
	osSignals := make([]os.Signal, len(signals))
	for i, sig := range signals {
		osSignals[i] = os.Signal(sig)
	}
	return signal.NotifyContext(ctx, osSignals...)
}

// Shutdown is a function that can be used to terminate the program.
func Shutdown() error {
	for _, sig := range ShutdownSignals {
		if err := syscall.Kill(syscall.Getpid(), sig); err == nil {
			return nil
		}
	}
	return fmt.Errorf("cannot shutdown gracefully")
}
