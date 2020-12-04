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
