package compute

import (
	"context"
	"errors"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/provider"
)

type mockLLM struct {
	shouldFail bool
	name       string
}

func (m *mockLLM) Name() string { return m.name }

func (m *mockLLM) Health(_ context.Context) error { return nil }

func (m *mockLLM) Models(_ context.Context) ([]provider.Model, error) {
	return []provider.Model{{ID: "qwen"}}, nil
}

func (m *mockLLM) Chat(_ context.Context, _ provider.ChatRequest) (provider.ChatResponse, error) {
	if m.shouldFail {
		return provider.ChatResponse{}, errors.New("backend timeout / connection refused")
	}
	return provider.ChatResponse{Content: "Inference response from " + m.name}, nil
}

func TestNodeRegistryAndLeastLoadedSelection(t *testing.T) {
	reg := NewNodeRegistry()

	reg.RegisterNode(Node{
		ID:             "vm-gpu-01",
		Host:           "10.0.10.11",
		GPUModel:       "NVIDIA A100",
		VRAMTotalMB:    81920,
		VRAMFreeMB:     40000,
		ActiveRequests: 5,
		Healthy:        true,
	})

	reg.RegisterNode(Node{
		ID:             "vm-gpu-02",
		Host:           "10.0.10.12",
		GPUModel:       "NVIDIA A100",
		VRAMTotalMB:    81920,
		VRAMFreeMB:     70000,
		ActiveRequests: 1,
		Healthy:        true,
	})

	best, err := reg.PickLeastLoadedNode()
	if err != nil {
		t.Fatalf("PickLeastLoadedNode error: %v", err)
	}
	if best.ID != "vm-gpu-02" {
		t.Errorf("Expected vm-gpu-02 (least active requests), got %s", best.ID)
	}

	// Test heartbeat marking vm-gpu-02 unhealthy
	if err := reg.Heartbeat("vm-gpu-02", 0, 100, 10, false); err != nil {
		t.Fatalf("Heartbeat error: %v", err)
	}

	bestAfterUnhealthy, err := reg.PickLeastLoadedNode()
	if err != nil {
		t.Fatalf("PickLeastLoadedNode error: %v", err)
	}
	if bestAfterUnhealthy.ID != "vm-gpu-01" {
		t.Errorf("Expected vm-gpu-01 after vm-gpu-02 became unhealthy, got %s", bestAfterUnhealthy.ID)
	}
}

func TestBackpressureQueueRejection(t *testing.T) {
	queue := NewBackpressureQueue(1, 1) // 1 in-flight, 1 in queue

	rel1, err := queue.TryAcquire()
	if err != nil {
		t.Fatalf("First acquisition failed: %v", err)
	}

	rel2, err := queue.TryAcquire()
	if err != nil {
		t.Fatalf("Second acquisition (queue) failed: %v", err)
	}

	// Third acquisition should be rejected by back-pressure
	_, err = queue.TryAcquire()
	if !errors.Is(err, ErrOverloaded) {
		t.Errorf("Expected ErrOverloaded on queue overflow, got %v", err)
	}

	rel1()
	rel2()
}

func TestHealthAwareRouterFailover(t *testing.T) {
	primary := &mockLLM{name: "primary-vllm", shouldFail: true}
	secondary := &mockLLM{name: "secondary-nim", shouldFail: false}

	router := NewHealthAwareRouter(primary, secondary, nil, nil)
	resp, err := router.Chat(context.Background(), provider.ChatRequest{
		Model:    "qwen",
		Messages: []provider.Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("Chat failover failed: %v", err)
	}

	if resp.Content != "Inference response from secondary-nim" {
		t.Errorf("Unexpected response content: %s", resp.Content)
	}
}
