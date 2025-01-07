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

package events

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

// EventType is the type used to define the reason of an event.
type EventType string

const (
	// Normal is the default reason to use.
	Normal EventType = "Normal"
	// Warning is the default reason to use.
	Warning EventType = "Warning"
	// Error is the default reason to use.
	Error EventType = "Error"
)

// Reason is the type used to define the reason of an event.
type Reason string

const (
	// Processing is the default reason to use.
	Processing Reason = "Processing"
)

// Option is the type used to define the options of an event.
type Option struct {
	Reason    Reason
	EventType EventType
}

// Event is a wrapper around the Event method of the EventRecorder interface.
// It uses the default EventType and Reason.
func Event(er record.EventRecorder, obj runtime.Object, message string) {
	er.Event(obj, string(Normal), string(Processing), message)
}

// EventWithOptions is a wrapper around the Event method of the EventRecorder interface.
// It uses the EventType and Reason passed as parameters in options.
func EventWithOptions(er record.EventRecorder, obj runtime.Object, message string, options *Option) {
	er.Event(obj, string(options.EventType), string(options.Reason), message)
}
