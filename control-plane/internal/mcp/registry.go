package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Registration is a configured server as the platform holds it.
type Registration struct {
	Slug              string            `json:"slug"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	BaseURL           string            `json:"baseUrl"`
	RequiredScope     string            `json:"requiredScope"`
	AllowedTools      []string          `json:"allowedTools,omitempty"`
	CompanyID         string            `json:"companyId,omitempty"`
	MaxCallsPerMinute int               `json:"maxCallsPerMinute"`
	Timeout           time.Duration     `json:"-"`
	LastDiscoveredAt  *time.Time        `json:"lastDiscoveredAt,omitempty"`
	DiscoveredTools   int               `json:"discoveredTools"`
	LastError         string            `json:"lastError,omitempty"`
	headers           map[string]string `json:"-"`
}

// Server builds the connection details, including the credentials that are never
// exposed on the Registration itself.
func (r Registration) Server() Server {
	return Server{
		Slug: r.Slug, Name: r.Name, BaseURL: r.BaseURL,
		Headers: r.headers, Timeout: r.Timeout,
	}
}

// Permits reports whether a remote tool name is allowed. An empty allowlist
// accepts everything the server advertises, which suits development and is too
// permissive for production - a server could otherwise widen its own surface
// after approval simply by advertising more tools.
func (r Registration) Permits(name string) bool {
	if len(r.AllowedTools) == 0 {
		return true
	}
	for _, allowed := range r.AllowedTools {
		if allowed == name {
			return true
		}
	}
	return false
}

// Store reads and updates the server registry.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) List(ctx context.Context) ([]Registration, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT slug, name, description, base_url, required_scope, headers, allowed_tools,
		       coalesce(company_id, ''), max_calls_per_minute, timeout_seconds,
		       last_discovered_at, discovered_tools, coalesce(last_error, '')
		FROM mcp_servers
		WHERE deleted_at IS NULL AND enabled
		ORDER BY slug`)
	if err != nil {
		return nil, fmt.Errorf("read mcp servers: %w", err)
	}
	defer rows.Close()

	var registrations []Registration
	for rows.Next() {
		var registration Registration
		var timeoutSeconds int
		if err := rows.Scan(&registration.Slug, &registration.Name, &registration.Description,
			&registration.BaseURL, &registration.RequiredScope, &registration.headers,
			&registration.AllowedTools, &registration.CompanyID, &registration.MaxCallsPerMinute,
			&timeoutSeconds, &registration.LastDiscoveredAt, &registration.DiscoveredTools,
			&registration.LastError); err != nil {
			return nil, fmt.Errorf("read mcp servers: %w", err)
		}
		registration.Timeout = time.Duration(timeoutSeconds) * time.Second
		registrations = append(registrations, registration)
	}
	return registrations, rows.Err()
}

// RecordDiscovery stores the outcome on the row, so an operator can see why a
// server's tools are missing without reading logs.
func (s *Store) RecordDiscovery(ctx context.Context, slug string, tools int, failure error) {
	reason := ""
	if failure != nil {
		reason = failure.Error()
	}
	_, _ = s.pool.Exec(ctx, `
		UPDATE mcp_servers
		SET last_discovered_at = now(), discovered_tools = $2,
		    last_error = nullif($3, ''), updated_at = now()
		WHERE slug = $1 AND deleted_at IS NULL`, slug, tools, reason)
}

// Discovered is one remote tool, resolved and ready to register.
type Discovered struct {
	Server Registration
	Client *Client
	Tool   Tool
	// Slug is namespaced by server, so two servers may both offer "search".
	Slug string
}

// Discover connects to each registered server and lists its tools.
//
// A server that cannot be reached is skipped with its reason recorded, not
// treated as fatal: an MCP server is a separate deployable and the platform must
// start without it. That is the same rule the compute plane follows.
func Discover(ctx context.Context, store *Store, log *slog.Logger) ([]Discovered, error) {
	registrations, err := store.List(ctx)
	if err != nil {
		return nil, err
	}

	var discovered []Discovered
	for _, registration := range registrations {
		client := NewClient(registration.Server())

		tools, err := client.ListTools(ctx)
		if err != nil {
			store.RecordDiscovery(ctx, registration.Slug, 0, err)
			log.Error("mcp discovery failed", "server", registration.Slug, "error", err)
			continue
		}

		accepted := 0
		for _, remote := range tools {
			if !registration.Permits(remote.Name) {
				log.Info("mcp tool not in allowlist", "server", registration.Slug, "tool", remote.Name)
				continue
			}
			discovered = append(discovered, Discovered{
				Server: registration, Client: client, Tool: remote,
				Slug: registration.Slug + "." + remote.Name,
			})
			accepted++
		}

		store.RecordDiscovery(ctx, registration.Slug, accepted, nil)
		log.Info("mcp server discovered",
			"server", registration.Slug, "advertised", len(tools), "registered", accepted)
	}
	return discovered, nil
}
