package utils

import "context"

type contextKey string

const RequestIDKey contextKey = "request_id"

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	value, ok := ctx.Value(RequestIDKey).(string)
	if !ok {
		return ""
	}
	return value
}
