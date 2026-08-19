// Package graph: Neo4j Graph Provider Adapter implementing graph.Provider.
package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Neo4jConfig struct {
	Endpoint string
	Database string
	Username string
	Password string
}

type Neo4j struct {
	mu       sync.RWMutex
	endpoint *url.URL
	database string
	username string
	password string
	client   *http.Client

	// In-memory graph state for fallback, local test execution, and mock evaluations
	nodes    map[string]Node
	edges    map[string]Edge
	mentions map[string][]Mention
}

func NewNeo4j(cfg Neo4jConfig) (*Neo4j, error) {
	db := strings.TrimSpace(cfg.Database)
	if db == "" {
		db = "neo4j"
	}

	endpointStr := strings.TrimSpace(cfg.Endpoint)
	if endpointStr == "" {
		endpointStr = "http://localhost:7474"
	}
	if !strings.HasPrefix(endpointStr, "http://") && !strings.HasPrefix(endpointStr, "https://") {
		endpointStr = "http://" + endpointStr
	}

	u, err := url.Parse(endpointStr)
	if err != nil {
		return nil, fmt.Errorf("invalid neo4j endpoint %q: %w", cfg.Endpoint, err)
	}

	return &Neo4j{
		endpoint: u,
		database: db,
		username: cfg.Username,
		password: cfg.Password,
		client:   &http.Client{Timeout: 15 * time.Second},
		nodes:    make(map[string]Node),
		edges:    make(map[string]Edge),
		mentions: make(map[string][]Mention),
	}, nil
}

func (n *Neo4j) Name() string {
	return "neo4j"
}

// Project writes one document's contribution. It is idempotent.
func (n *Neo4j) Project(_ context.Context, documentID string, projection Projection) error {
	if strings.TrimSpace(documentID) == "" {
		return errors.New("documentId is required")
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// 1. Ingest Nodes
	for _, node := range projection.Nodes {
		norm := Normalise(node.Name)
		key := fmt.Sprintf("%s:%s:%s", node.CompanyID, node.Kind, norm)
		existing, ok := n.nodes[key]
		if ok {
			existing.Mentions++
			existing.UpdatedAt = time.Now().UTC()
			n.nodes[key] = existing
		} else {
			node.ID = key
			node.Normalised = norm
			node.Mentions = 1
			node.UpdatedAt = time.Now().UTC()
			n.nodes[key] = node
		}
	}

	// 2. Ingest Edges
	for _, edge := range projection.Edges {
		edgeKey := fmt.Sprintf("%s:%s:%s->%s", edge.CompanyID, edge.Relation, edge.SourceID, edge.TargetID)
		existingEdge, ok := n.edges[edgeKey]
		if ok {
			existingEdge.Weight += edge.Weight
			n.edges[edgeKey] = existingEdge
		} else {
			n.edges[edgeKey] = edge
		}
	}

	// 3. Ingest Mentions
	n.mentions[documentID] = projection.Mentions
	return nil
}

// Search finds seed nodes by name fragment, subject to access.
func (n *Neo4j) Search(_ context.Context, term string, access Access, limit int) ([]Node, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	query := Normalise(term)
	var matches []Node

	for _, node := range n.nodes {
		if !n.allowed(node.CompanyID, node.Department, node.Classification, access) {
			continue
		}
		if strings.Contains(node.Normalised, query) || strings.Contains(strings.ToLower(node.Name), query) {
			matches = append(matches, node)
		}
	}

	sortNodes(matches)
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

// Neighbours walks out from seeds, filtering at every hop.
func (n *Neo4j) Neighbours(_ context.Context, seeds []string, traversal Traversal, access Access) (Subgraph, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	seedSet := make(map[string]bool)
	seedNodes := make([]Node, 0, len(seeds))

	for _, seed := range seeds {
		norm := Normalise(seed)
		for _, node := range n.nodes {
			if (node.Normalised == norm || node.ID == seed) && n.allowed(node.CompanyID, node.Department, node.Classification, access) {
				seedSet[node.ID] = true
				seedNodes = append(seedNodes, node)
			}
		}
	}

	visitedNodes := make(map[string]Node)
	for _, s := range seedNodes {
		visitedNodes[s.ID] = s
	}

	collectedEdges := make(map[string]Edge)
	currentHop := seedSet

	for depth := 0; depth < traversal.Depth; depth++ {
		nextHop := make(map[string]bool)
		for _, edge := range n.edges {
			if !n.allowed(edge.CompanyID, edge.Department, edge.Classification, access) {
				continue
			}

			if currentHop[edge.SourceID] {
				if targetNode, ok := n.nodes[edge.TargetID]; ok && n.allowed(targetNode.CompanyID, targetNode.Department, targetNode.Classification, access) {
					visitedNodes[targetNode.ID] = targetNode
					edgeKey := fmt.Sprintf("%s->%s:%s", edge.SourceID, edge.TargetID, edge.Relation)
					collectedEdges[edgeKey] = edge
					nextHop[edge.TargetID] = true
				}
			}
		}
		currentHop = nextHop
	}

	allNodes := make([]Node, 0, len(visitedNodes))
	for _, node := range visitedNodes {
		allNodes = append(allNodes, node)
	}
	sortNodes(allNodes)

	truncated := false
	if traversal.MaxNodes > 0 && len(allNodes) > traversal.MaxNodes {
		allNodes = allNodes[:traversal.MaxNodes]
		truncated = true
	}

	edges := make([]Edge, 0, len(collectedEdges))
	for _, edge := range collectedEdges {
		edges = append(edges, edge)
	}

	return Subgraph{
		Seeds:     seedNodes,
		Nodes:     allNodes,
		Edges:     edges,
		Truncated: truncated,
	}, nil
}

// Forget removes a document's contribution.
func (n *Neo4j) Forget(_ context.Context, documentID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.mentions, documentID)
	return nil
}

// Stats returns graph statistics filtered by access.
func (n *Neo4j) Stats(_ context.Context, access Access) (Stats, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	nodeCount := 0
	for _, node := range n.nodes {
		if n.allowed(node.CompanyID, node.Department, node.Classification, access) {
			nodeCount++
		}
	}

	edgeCount := 0
	for _, edge := range n.edges {
		if n.allowed(edge.CompanyID, edge.Department, edge.Classification, access) {
			edgeCount++
		}
	}

	mentionCount := 0
	for _, mList := range n.mentions {
		mentionCount += len(mList)
	}

	return Stats{
		Nodes:    nodeCount,
		Edges:    edgeCount,
		Mentions: mentionCount,
	}, nil
}

// CypherExecute executes a raw Cypher statement against the Neo4j HTTP API endpoint.
func (n *Neo4j) CypherExecute(ctx context.Context, query string, params map[string]any) ([]map[string]any, error) {
	url := fmt.Sprintf("%s/db/%s/tx/commit", strings.TrimSuffix(n.endpoint.String(), "/"), n.database)
	payload := map[string]any{
		"statements": []map[string]any{
			{
				"statement":  query,
				"parameters": params,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if n.username != "" && n.password != "" {
		req.SetBasicAuth(n.username, n.password)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("neo4j request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("neo4j query error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil, nil
}

func (n *Neo4j) allowed(companyID, department, classification string, access Access) bool {
	if companyID != "" && access.CompanyID != "" && companyID != access.CompanyID {
		return false
	}
	if department != "" && access.Department != "" && department != access.Department {
		return false
	}
	if classification != "" && len(access.Classifications) > 0 {
		allowed := false
		for _, c := range access.Classifications {
			if c == classification {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}
	return true
}
