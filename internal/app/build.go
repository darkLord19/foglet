// Package app is the composition root for Fog.
package app

import (
	"context"
	"net/http"

	"github.com/darkLord19/foglet/internal/api"
	"github.com/darkLord19/foglet/internal/runner"
	"github.com/darkLord19/foglet/internal/state"
)

// App is the fully-wired application graph.
type App struct {
	Handler http.Handler // fully-wired middleware stack (auth + CORS + body limit + routes)
	Runner  *runner.Runner
	Store   *state.Store
	mux     *http.ServeMux // for Mount()
}

// BuildOpts configures the application.
type BuildOpts struct {
	FogHome string // the fog home directory (~/.fog)
	Cwd     string // working directory / repo root
	Port    int    // API server port
}

// Build constructs the full application graph and returns it.
// Callers must call Close() when done.
func Build(ctx context.Context, opts BuildOpts) (*App, error) {
	// 1. Create state store
	store, err := state.NewStore(opts.FogHome)
	if err != nil {
		return nil, err
	}

	// 2. Create runner with state store
	r := runner.New(store)
	r.SetBaseContext(ctx)

	// 3. Create API server
	apiServer := api.New(r, store, opts.Port)

	// 4. Register routes on mux
	mux := http.NewServeMux()
	apiServer.RegisterRoutes(mux)

	// 5. Generate API token and write to file (for desktop UI)
	apiToken, err := api.GenerateAPIToken()
	if err != nil {
		store.Close()
		return nil, err
	}
	if err := api.WriteTokenFile(opts.FogHome, apiToken); err != nil {
		store.Close()
		return nil, err
	}

	// 6. Build middleware chain
	handler := api.WithCORS(api.WithBodyLimit(api.WithAuth(apiToken, mux)))

	return &App{
		Handler: handler,
		Runner:  r,
		Store:   store,
		mux:     mux,
	}, nil
}

// Mount registers an additional handler under the given prefix.
// Routes added via Mount go through the same middleware chain as api routes.
// This is the single attach point for MCP handlers, Slack webhooks, etc.
func (a *App) Mount(prefix string, h http.Handler) {
	a.mux.Handle(prefix, h)
}

// Close cleans up resources (db connection, etc).
func (a *App) Close() {
	if a.Store != nil {
		_ = a.Store.Close()
	}
}
