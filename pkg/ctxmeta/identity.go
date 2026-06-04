package ctxmeta

import (
	"context"

	"git.woa.com/leondli/workspace-service/internal/domain/identity"
)

type requestContextKey struct{}

// WithRequestContext stores identity on the context (e.g. after HTTP bind).
func WithRequestContext(ctx context.Context, rc identity.RequestContext) context.Context {
	return context.WithValue(ctx, requestContextKey{}, rc.Normalize())
}

// RequestContextFrom returns the identity stored on the context, if any.
func RequestContextFrom(ctx context.Context) (identity.RequestContext, bool) {
	rc, ok := ctx.Value(requestContextKey{}).(identity.RequestContext)
	return rc, ok
}
