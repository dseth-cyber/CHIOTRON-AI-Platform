package auth

import "context"

type contextKey struct{}

// WithIdentity attaches the authenticated caller to a request context.
func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

// IdentityFrom returns the authenticated caller, if the request passed through
// the authentication middleware.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	return identity, ok
}
