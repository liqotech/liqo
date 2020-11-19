package context

import "context"

type contextKey string

const (
	callingKey contextKey = "calling"
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
