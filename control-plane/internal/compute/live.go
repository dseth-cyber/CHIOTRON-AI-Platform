package compute

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/chiotron/ai-control-plane/internal/provider"
)

// Live is a routing registry that can be rebuilt while the process serves.
//
// ARCHITECTURE-v1 section 46 wants routing to be configuration an operator
// changes from the Admin UI. Configuration that needs a restart is
// configuration nobody touches in production, so the pointer is swapped under
// an atomic and in-flight requests finish against the registry they started
// with.
//
// It satisfies the same surfaces the rest of the platform already depends on,
// so nothing downstream knows the table can change underneath it.
type Live struct {
	store   *Store
	log     *slog.Logger
	current atomic.Pointer[provider.Registry]
}

func NewLive(store *Store, log *slog.Logger) *Live {
	return &Live{store: store, log: log}
}

// Reload rebuilds from the database.
//
// A failure leaves the previous registry in place. A bad edit should be
// reported and ignored, not applied by emptying the routing table.
func (l *Live) Reload(ctx context.Context) error {
	rebuilt, err := l.store.Build(ctx, l.log)
	if err != nil {
		return err
	}
	l.current.Store(rebuilt)
	l.log.Info("compute registry reloaded",
		"defaultModel", rebuilt.DefaultModel(), "routes", len(rebuilt.Routes()))
	return nil
}

// Registry returns the registry currently in force.
func (l *Live) Registry() *provider.Registry { return l.current.Load() }

func (l *Live) Resolve(logical string) (provider.LLM, provider.Route, error) {
	return l.Registry().Resolve(logical)
}

func (l *Live) Chat(ctx context.Context, logical string, req provider.ChatRequest) (provider.ChatResponse, provider.Route, error) {
	return l.Registry().Chat(ctx, logical, req)
}

func (l *Live) ChatStream(ctx context.Context, logical string, req provider.ChatRequest,
	emit func(provider.Chunk) error) (provider.ChatResponse, provider.Route, error) {
	return l.Registry().ChatStream(ctx, logical, req, emit)
}

func (l *Live) DefaultModel() string      { return l.Registry().DefaultModel() }
func (l *Live) Routes() []provider.Route  { return l.Registry().Routes() }
func (l *Live) Providers() []provider.LLM { return l.Registry().Providers() }
