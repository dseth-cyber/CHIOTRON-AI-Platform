package graph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres keeps the graph in AI-owned tables, which is where GraphRAG starts so
// that no infrastructure decision has to be made before there is a graph worth
// running on (ARCHITECTURE-v1 section 6).
type Postgres struct {
	pool *pgxpool.Pool
	// ladder is the classification order, least sensitive first. It is needed in
	// SQL to compare two levels when documents disagree about a shared node.
	ladder []string
}

func NewPostgres(pool *pgxpool.Pool, ladder []string) *Postgres {
	return &Postgres{pool: pool, ladder: ladder}
}

func (*Postgres) Name() string { return "postgres" }

const nodeColumns = `id::text, kind, name, normalised_name, classification,
	coalesce(company_id, ''), coalesce(department, ''), properties, mention_count, updated_at`

// Project replaces a document's contribution.
//
// Forgetting first is what makes re-ingestion idempotent: without it a second
// pass would double every weight and every mention count, and the graph would
// quietly report a relationship as twice as strong as the text supports.
func (p *Postgres) Project(ctx context.Context, documentID string, projection Projection) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin projection: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if err := forget(ctx, tx, documentID); err != nil {
		return err
	}

	// Nodes are shared across documents, so this upserts rather than inserts.
	identifiers := make(map[string]string, len(projection.Nodes))
	for _, node := range projection.Nodes {
		properties, err := json.Marshal(orEmpty(node.Properties))
		if err != nil {
			return fmt.Errorf("encode node properties: %w", err)
		}

		var id string
		// A node several documents mention keeps the *least* sensitive
		// classification among them. Knowing that an entity exists is only as
		// sensitive as the least restricted document that says so, and taking the
		// first writer's level instead would hide an entity from readers whose own
		// documents mention it. Edges keep their own document's level, which is
		// what actually stops a walk from reaching restricted material.
		err = tx.QueryRow(ctx, `
			INSERT INTO graph_nodes (kind, name, normalised_name, properties,
			                         classification, company_id, department)
			VALUES ($1, $2, $3, $4, $5, nullif($6, ''), nullif($7, ''))
			ON CONFLICT (kind, normalised_name, coalesce(company_id, ''))
			  WHERE deleted_at IS NULL
			DO UPDATE SET name = excluded.name,
			              properties = graph_nodes.properties || excluded.properties,
			              classification = CASE
			                WHEN coalesce(array_position($8::text[], excluded.classification), 2147483647)
			                   < coalesce(array_position($8::text[], graph_nodes.classification), 2147483647)
			                THEN excluded.classification
			                ELSE graph_nodes.classification
			              END,
			              updated_at = now()
			RETURNING id::text`,
			node.Kind, node.Name, node.Normalised, properties,
			node.Classification, node.CompanyID, node.Department, p.ladder).Scan(&id)
		if err != nil {
			return fmt.Errorf("upsert node %q: %w", node.Normalised, err)
		}
		identifiers[node.Normalised] = id
	}

	for _, edge := range projection.Edges {
		source, sourceKnown := identifiers[edge.SourceID]
		target, targetKnown := identifiers[edge.TargetID]
		if !sourceKnown || !targetKnown {
			// An edge naming a node the projection did not include would create a
			// dangling relationship; the extractor is expected to be consistent.
			return fmt.Errorf("edge %s -> %s (%s) names a node outside the projection",
				edge.SourceID, edge.TargetID, edge.Relation)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO graph_edges (source_id, target_id, relation, document_id, weight,
			                         classification, company_id, department)
			VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5, $6, nullif($7, ''), nullif($8, ''))
			ON CONFLICT (source_id, target_id, relation, document_id)
			DO UPDATE SET weight = excluded.weight, updated_at = now()`,
			source, target, edge.Relation, documentID, edge.Weight,
			edge.Classification, edge.CompanyID, edge.Department); err != nil {
			return fmt.Errorf("upsert edge %s: %w", edge.Relation, err)
		}
	}

	for _, mention := range projection.Mentions {
		id, known := identifiers[mention.NodeName]
		if !known {
			return fmt.Errorf("mention names node %q outside the projection", mention.NodeName)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO graph_mentions (node_id, document_id, chunk_ordinal, occurrences)
			VALUES ($1::uuid, $2::uuid, $3, $4)
			ON CONFLICT (node_id, document_id, chunk_ordinal)
			DO UPDATE SET occurrences = excluded.occurrences`,
			id, mention.DocumentID, mention.ChunkOrdinal, mention.Occurrences); err != nil {
			return fmt.Errorf("upsert mention: %w", err)
		}
	}

	if err := recountMentions(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit projection: %w", err)
	}
	return nil
}

// Forget removes what one document contributed, and any node left with nothing
// pointing at it.
func (p *Postgres) Forget(ctx context.Context, documentID string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin forget: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if err := forget(ctx, tx, documentID); err != nil {
		return err
	}
	if err := recountMentions(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit forget: %w", err)
	}
	return nil
}

func forget(ctx context.Context, tx pgx.Tx, documentID string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM graph_edges WHERE document_id = $1::uuid`, documentID); err != nil {
		return fmt.Errorf("clear document edges: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM graph_mentions WHERE document_id = $1::uuid`, documentID); err != nil {
		return fmt.Errorf("clear document mentions: %w", err)
	}
	// A node with no mentions and no edges is no longer evidenced by anything, so
	// leaving it would let a withdrawn document keep answering questions.
	if _, err := tx.Exec(ctx, `
		DELETE FROM graph_nodes n
		WHERE NOT EXISTS (SELECT 1 FROM graph_mentions m WHERE m.node_id = n.id)
		  AND NOT EXISTS (SELECT 1 FROM graph_edges e WHERE e.source_id = n.id OR e.target_id = n.id)`); err != nil {
		return fmt.Errorf("clear orphaned nodes: %w", err)
	}
	return nil
}

// recountMentions derives mention_count from the mention rows rather than
// maintaining a counter, so the number cannot drift from its evidence.
func recountMentions(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `
		UPDATE graph_nodes n
		SET mention_count = coalesce((
			SELECT sum(m.occurrences) FROM graph_mentions m WHERE m.node_id = n.id
		), 0)`); err != nil {
		return fmt.Errorf("recount mentions: %w", err)
	}
	return nil
}

// Search finds seed nodes. The access predicates are in SQL so a node the caller
// may not read is never a seed.
func (p *Postgres) Search(ctx context.Context, term string, access Access, limit int) ([]Node, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := p.pool.Query(ctx, `
		SELECT `+nodeColumns+`
		FROM graph_nodes
		WHERE deleted_at IS NULL
		  AND normalised_name LIKE '%' || $1 || '%'
		  AND (company_id IS NULL OR company_id = nullif($2, ''))
		  AND (department IS NULL OR department = nullif($3, ''))
		  AND classification = ANY($4)
		ORDER BY mention_count DESC, normalised_name
		LIMIT $5`, Normalise(term), access.CompanyID, access.Department, access.Classifications, limit)
	if err != nil {
		return nil, fmt.Errorf("search graph nodes: %w", err)
	}
	defer rows.Close()

	return scanNodes(rows)
}

// Neighbours walks out from the seeds, one hop at a time.
//
// The access predicates are applied at every hop rather than only to the seeds.
// A readable node must not become a bridge to an unreadable one: that is exactly
// how a graph leaks what a document ACL was meant to protect.
func (p *Postgres) Neighbours(ctx context.Context, seeds []string, traversal Traversal, access Access) (Subgraph, error) {
	if len(seeds) == 0 {
		return Subgraph{}, nil
	}

	visited := make(map[string]Node, traversal.MaxNodes)
	frontier := append([]string(nil), seeds...)
	var subgraph Subgraph

	seedNodes, err := p.nodesByID(ctx, seeds, access)
	if err != nil {
		return Subgraph{}, err
	}
	subgraph.Seeds = seedNodes
	for _, node := range seedNodes {
		visited[node.ID] = node
	}
	// A seed the caller may not read is dropped here, so it cannot seed a walk.
	frontier = frontier[:0]
	for _, node := range seedNodes {
		frontier = append(frontier, node.ID)
	}

	relations := traversal.Relations
	for depth := 0; depth < traversal.Depth && len(frontier) > 0; depth++ {
		edges, err := p.edgesFrom(ctx, frontier, relations, access)
		if err != nil {
			return Subgraph{}, err
		}
		subgraph.Edges = append(subgraph.Edges, edges...)

		var next []string
		for _, edge := range edges {
			for _, candidate := range []string{edge.SourceID, edge.TargetID} {
				if _, seen := visited[candidate]; seen {
					continue
				}
				if len(visited) >= traversal.MaxNodes {
					subgraph.Truncated = true
					continue
				}
				// Reserve the slot before the lookup so the cap holds even when a
				// hop returns more candidates than remain.
				visited[candidate] = Node{}
				next = append(next, candidate)
			}
		}
		if len(next) == 0 {
			break
		}
		found, err := p.nodesByID(ctx, next, access)
		if err != nil {
			return Subgraph{}, err
		}
		// Candidates the access predicates removed are not part of the graph the
		// caller sees, and must not be walked further.
		frontier = frontier[:0]
		for _, node := range found {
			visited[node.ID] = node
			frontier = append(frontier, node.ID)
		}
		for _, candidate := range next {
			if node := visited[candidate]; node.ID == "" {
				delete(visited, candidate)
			}
		}
	}

	for _, node := range visited {
		if node.ID != "" {
			subgraph.Nodes = append(subgraph.Nodes, node)
		}
	}
	sortNodes(subgraph.Nodes)
	subgraph.Edges = keepEdgesWithin(subgraph.Edges, visited)
	return subgraph, nil
}

func (p *Postgres) nodesByID(ctx context.Context, ids []string, access Access) ([]Node, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT `+nodeColumns+`
		FROM graph_nodes
		WHERE deleted_at IS NULL
		  AND id = ANY($1::uuid[])
		  AND (company_id IS NULL OR company_id = nullif($2, ''))
		  AND (department IS NULL OR department = nullif($3, ''))
		  AND classification = ANY($4)`,
		ids, access.CompanyID, access.Department, access.Classifications)
	if err != nil {
		return nil, fmt.Errorf("read graph nodes: %w", err)
	}
	defer rows.Close()

	return scanNodes(rows)
}

// edgesFrom returns one hop, aggregating per-document contributions into a total
// weight for the relationship.
func (p *Postgres) edgesFrom(ctx context.Context, from []string, relations []string, access Access) ([]Edge, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT e.source_id::text, e.target_id::text, e.relation, sum(e.weight)::int,
		       min(e.classification), coalesce(min(e.company_id), ''), coalesce(min(e.department), '')
		FROM graph_edges e
		WHERE (e.source_id = ANY($1::uuid[]) OR e.target_id = ANY($1::uuid[]))
		  AND ($2::text[] IS NULL OR cardinality($2::text[]) = 0 OR e.relation = ANY($2::text[]))
		  AND (e.company_id IS NULL OR e.company_id = nullif($3, ''))
		  AND (e.department IS NULL OR e.department = nullif($4, ''))
		  AND e.classification = ANY($5)
		GROUP BY e.source_id, e.target_id, e.relation
		ORDER BY sum(e.weight) DESC
		LIMIT 500`, from, relations, access.CompanyID, access.Department, access.Classifications)
	if err != nil {
		return nil, fmt.Errorf("read graph edges: %w", err)
	}
	defer rows.Close()

	var edges []Edge
	for rows.Next() {
		var edge Edge
		if err := rows.Scan(&edge.SourceID, &edge.TargetID, &edge.Relation, &edge.Weight,
			&edge.Classification, &edge.CompanyID, &edge.Department); err != nil {
			return nil, fmt.Errorf("read graph edges: %w", err)
		}
		edges = append(edges, edge)
	}
	return edges, rows.Err()
}

func (p *Postgres) Stats(ctx context.Context, access Access) (Stats, error) {
	var stats Stats
	err := p.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM graph_nodes n
		        WHERE n.deleted_at IS NULL
		          AND (n.company_id IS NULL OR n.company_id = nullif($1, ''))
		          AND n.classification = ANY($2)),
		       (SELECT count(*) FROM graph_edges e
		        WHERE (e.company_id IS NULL OR e.company_id = nullif($1, ''))
		          AND e.classification = ANY($2)),
		       (SELECT count(*) FROM graph_mentions)`,
		access.CompanyID, access.Classifications).Scan(&stats.Nodes, &stats.Edges, &stats.Mentions)
	if err != nil {
		return Stats{}, fmt.Errorf("read graph stats: %w", err)
	}
	return stats, nil
}

func scanNodes(rows pgx.Rows) ([]Node, error) {
	var nodes []Node
	for rows.Next() {
		var node Node
		var properties []byte
		if err := rows.Scan(&node.ID, &node.Kind, &node.Name, &node.Normalised, &node.Classification,
			&node.CompanyID, &node.Department, &properties, &node.Mentions, &node.UpdatedAt); err != nil {
			return nil, fmt.Errorf("read graph node: %w", err)
		}
		if len(properties) > 0 {
			_ = json.Unmarshal(properties, &node.Properties)
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

// keepEdgesWithin drops edges to nodes the caller could not read, so the returned
// subgraph never hints at what was filtered out.
func keepEdgesWithin(edges []Edge, visited map[string]Node) []Edge {
	kept := make([]Edge, 0, len(edges))
	for _, edge := range edges {
		source, sourceOK := visited[edge.SourceID]
		target, targetOK := visited[edge.TargetID]
		if sourceOK && targetOK && source.ID != "" && target.ID != "" {
			kept = append(kept, edge)
		}
	}
	return kept
}

func orEmpty(properties map[string]any) map[string]any {
	if properties == nil {
		return map[string]any{}
	}
	return properties
}
