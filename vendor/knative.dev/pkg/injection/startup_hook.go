package injection

import "golang.org/x/net/context"

type startupHookKey struct{}

// WithNamespaceScope associates a namespace scoping with the
// provided context, which will scope the informers produced
// by the downstream informer factories.
func WithStartupHook(ctx context.Context, hook func(context.Context) error) context.Context {
	hooks := GetStartupHooks(ctx)
	hooks = append(hooks, hook)
	return context.WithValue(ctx, startupHookKey{}, hooks)
}

// GetNamespaceScope accesses the namespace associated with the
// provided context.  This should be called when the injection
// logic is setting up shared informer factories.
func GetStartupHooks(ctx context.Context) []func(context.Context) error {
	value := ctx.Value(startupHookKey{})
	if value == nil {
		return []func(context.Context) error{}
	}
	return value.([]func(context.Context) error)
}
