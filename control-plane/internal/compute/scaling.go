// Package compute: Multi-Compute Scaling, GPU node registry, back-pressure policy, and health-aware routing.
package compute

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/chiotron/ai-control-plane/internal/provider"
)

var (
	ErrOverloaded     = errors.New("compute plane overloaded: request rejected by back-pressure queue")
	ErrNoHealthyNodes = errors.New("no healthy compute nodes available for inference")
	ErrNodeNotFound   = errors.New("compute node not found")
)

// Node represents an individual GPU VM or inference worker in the compute fleet.
type Node struct {
	ID             string    `json:"id"`
	Host           string    `json:"host"`
	GPUModel       string    `json:"gpuModel"`
	VRAMTotalMB    int       `json:"vramTotalMb"`
	VRAMFreeMB     int       `json:"vramFreeMb"`
	UtilizationPct float64   `json:"utilizationPct"`
	ActiveRequests int       `json:"activeRequests"`
	Healthy        bool      `json:"healthy"`
	Weight         int       `json:"weight"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// NodeRegistry tracks cluster nodes and their live GPU capacity.
type NodeRegistry struct {
	mu    sync.RWMutex
	nodes map[string]Node
}

// NewNodeRegistry initializes a new in-memory compute node registry.
func NewNodeRegistry() *NodeRegistry {
	return &NodeRegistry{
		nodes: make(map[string]Node),
	}
}

// RegisterNode adds or updates a compute node in the registry.
func (r *NodeRegistry) RegisterNode(node Node) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if node.Weight <= 0 {
		node.Weight = 1
	}
	if node.UpdatedAt.IsZero() {
		node.UpdatedAt = time.Now().UTC()
	}
	r.nodes[node.ID] = node
}

// Heartbeat updates the live health, utilization, and free VRAM of a node.
func (r *NodeRegistry) Heartbeat(nodeID string, vramFreeMB int, utilPct float64, activeReqs int, healthy bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	node, exists := r.nodes[nodeID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrNodeNotFound, nodeID)
	}

	node.VRAMFreeMB = vramFreeMB
	node.UtilizationPct = utilPct
	node.ActiveRequests = activeReqs
	node.Healthy = healthy
	node.UpdatedAt = time.Now().UTC()
	r.nodes[nodeID] = node
	return nil
}

// ListHealthyNodes returns all nodes currently reporting healthy status.
func (r *NodeRegistry) ListHealthyNodes() []Node {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var healthy []Node
	for _, n := range r.nodes {
		if n.Healthy {
			healthy = append(healthy, n)
		}
	}
	return healthy
}

// PickLeastLoadedNode selects the best node based on available VRAM and active load.
func (r *NodeRegistry) PickLeastLoadedNode() (Node, error) {
	healthy := r.ListHealthyNodes()
	if len(healthy) == 0 {
		return Node{}, ErrNoHealthyNodes
	}

	// Sort by ActiveRequests ascending, then VRAMFreeMB descending
	sort.Slice(healthy, func(i, j int) bool {
		if healthy[i].ActiveRequests != healthy[j].ActiveRequests {
			return healthy[i].ActiveRequests < healthy[j].ActiveRequests
		}
		return healthy[i].VRAMFreeMB > healthy[j].VRAMFreeMB
	})

	return healthy[0], nil
}

// BackpressureQueue protects GPU compute planes from OOM during traffic spikes.
type BackpressureQueue struct {
	mu          sync.Mutex
	maxInFlight int
	maxQueue    int
	inFlight    int
	queueDepth  int
}

// NewBackpressureQueue creates a queue manager with concurrency limits.
func NewBackpressureQueue(maxInFlight, maxQueue int) *BackpressureQueue {
	if maxInFlight <= 0 {
		maxInFlight = 20
	}
	if maxQueue <= 0 {
		maxQueue = 50
	}
	return &BackpressureQueue{
		maxInFlight: maxInFlight,
		maxQueue:    maxQueue,
	}
}

// TryAcquire attempts to acquire an execution slot or returns ErrOverloaded.
func (q *BackpressureQueue) TryAcquire() (func(), error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.inFlight < q.maxInFlight {
		q.inFlight++
		return q.releaseInFlight, nil
	}

	if q.queueDepth >= q.maxQueue {
		return nil, ErrOverloaded
	}

	q.queueDepth++
	return q.releaseQueue, nil
}

func (q *BackpressureQueue) releaseInFlight() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.inFlight > 0 {
		q.inFlight--
	}
}

func (q *BackpressureQueue) releaseQueue() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.queueDepth > 0 {
		q.queueDepth--
	}
}

// HealthAwareRouter wraps a collection of provider backends with automatic failover.
type HealthAwareRouter struct {
	primary   provider.LLM
	secondary provider.LLM
	registry  *NodeRegistry
	queue     *BackpressureQueue
}

// NewHealthAwareRouter creates a resilient model router.
func NewHealthAwareRouter(primary, secondary provider.LLM, reg *NodeRegistry, queue *BackpressureQueue) *HealthAwareRouter {
	return &HealthAwareRouter{
		primary:   primary,
		secondary: secondary,
		registry:  reg,
		queue:     queue,
	}
}

// Chat executes a chat inference with back-pressure protection and automatic secondary failover.
func (r *HealthAwareRouter) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	if r.queue != nil {
		release, err := r.queue.TryAcquire()
		if err != nil {
			return provider.ChatResponse{}, err
		}
		defer release()
	}

	// Try Primary
	resp, err := r.primary.Chat(ctx, req)
	if err == nil {
		return resp, nil
	}

	// If primary fails and secondary exists, failover gracefully
	if r.secondary != nil {
		return r.secondary.Chat(ctx, req)
	}

	return provider.ChatResponse{}, fmt.Errorf("primary inference failed: %w", err)
}
