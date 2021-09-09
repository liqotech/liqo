// Copyright 2019-2021 The Liqo Authors
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

package context

import "context"

type contextKey string

const (
	callingKey        contextKey = "calling"
	incomingMethodKey contextKey = "incomingMethod"
)

func (c contextKey) String() string {
	return string(c)
}

func CallingFunction(ctx context.Context) (string, bool) {
	tokenStr, ok := ctx.Value(callingKey).(string)
	return tokenStr, ok
}

func SetCallingFunction(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, callingKey, value)
}

func IncomingMethod(ctx context.Context) (string, bool) {
	tokenStr, ok := ctx.Value(incomingMethodKey).(string)
	return tokenStr, ok
}

func SetIncomingMethod(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, incomingMethodKey, value)
}
