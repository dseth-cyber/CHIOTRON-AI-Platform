package enterprise

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestERPAdapterReadAndWriteWorkflows(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Header propagation
		if r.Header.Get("X-Company-ID") != "acme" {
			t.Errorf("Missing or invalid X-Company-ID header: %s", r.Header.Get("X-Company-ID"))
		}
		if r.Header.Get("X-Caller-ID") != "user-123" {
			t.Errorf("Missing or invalid X-Caller-ID header: %s", r.Header.Get("X-Caller-ID"))
		}

		switch r.URL.Path {
		case "/api/v1/inventory":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"sku":"SKU-1001","name":"Valve A","quantity":50,"warehouse":"WH-01","unitPrice":1200.0}]`))
		case "/api/v1/orders/PO-100":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"PO-100","customerId":"CUST-1","total":5000.0,"status":"confirmed"}`))
		case "/api/v1/customers/CUST-1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"CUST-1","name":"Customer One","tier":"gold","creditLimit":100000.0}`))
		case "/api/v1/orders":
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":"PO-101","total":2400.0,"status":"pending"}`))
			}
		case "/api/v1/workflows/approvals":
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"requestId":"REQ-01","status":"approved","message":"PO approved within budget threshold"}`))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	adapter := NewHTTPERPAdapter(ts.URL, 5*time.Second)
	caller := CallerContext{
		CompanyID:  "acme",
		Department: "procurement",
		UserID:     "user-123",
		Clearance:  "internal",
	}

	ctx := context.Background()

	// 1. Get Inventory
	items, err := adapter.GetInventory(ctx, caller, "valve")
	if err != nil {
		t.Fatalf("GetInventory error: %v", err)
	}
	if len(items) != 1 || items[0].SKU != "SKU-1001" {
		t.Errorf("Unexpected inventory: %+v", items)
	}

	// 2. Get Order
	order, err := adapter.GetOrder(ctx, caller, "PO-100")
	if err != nil {
		t.Fatalf("GetOrder error: %v", err)
	}
	if order.Status != "confirmed" {
		t.Errorf("Unexpected order status: %s", order.Status)
	}

	// 3. Get Customer
	cust, err := adapter.GetCustomer(ctx, caller, "CUST-1")
	if err != nil {
		t.Fatalf("GetCustomer error: %v", err)
	}
	if cust.Tier != "gold" {
		t.Errorf("Unexpected customer tier: %s", cust.Tier)
	}

	// 4. Create Purchase Order (Authorized Write)
	newPO, err := adapter.CreatePurchaseOrder(ctx, caller, CreatePOParams{
		VendorID:    "VEND-1",
		TotalAmount: 2400.0,
		Description: "Spare parts",
	})
	if err != nil {
		t.Fatalf("CreatePurchaseOrder error: %v", err)
	}
	if newPO.ID != "PO-101" {
		t.Errorf("Unexpected PO ID: %s", newPO.ID)
	}

	// 5. Submit Approval Request (Authorized Write Workflow)
	appResult, err := adapter.SubmitApprovalRequest(ctx, caller, ApprovalRequestParams{
		WorkflowID:  "WF-PO-101",
		Amount:      2400.0,
		RequestedBy: "user-123",
		Reason:      "Urgent replacement",
	})
	if err != nil {
		t.Fatalf("SubmitApprovalRequest error: %v", err)
	}
	if appResult.Status != "approved" {
		t.Errorf("Unexpected approval result: %+v", appResult)
	}
}

func TestERPFailureIsolation(t *testing.T) {
	// Simulate ERP returning 500 internal errors / downtime
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"ERP database connection refused"}`))
	}))
	defer ts.Close()

	adapter := NewHTTPERPAdapter(ts.URL, 500*time.Millisecond)
	caller := CallerContext{CompanyID: "acme"}

	// Verify that error is caught as ErrERPUnavailable and does not panic or crash
	_, err := adapter.GetInventory(context.Background(), caller, "any")
	if err == nil {
		t.Fatal("Expected error on ERP downtime, got nil")
	}
	if !errors.Is(err, ErrERPUnavailable) {
		t.Errorf("Expected ErrERPUnavailable, got %v", err)
	}
}

func TestKafkaEventBusPublishAndSubscribe(t *testing.T) {
	bus := NewMemoryKafkaEventBus()
	ctx := context.Background()

	var receivedCount int32
	bus.Subscribe(EventOrderProcessed, func(event EnterpriseEvent) {
		atomic.AddInt32(&receivedCount, 1)
		if event.CompanyID != "acme" {
			t.Errorf("Unexpected company id in event: %s", event.CompanyID)
		}
	})

	err := bus.Publish(ctx, EnterpriseEvent{
		ID:        "evt-01",
		Topic:     EventOrderProcessed,
		CompanyID: "acme",
		Payload:   map[string]any{"orderId": "PO-101", "status": "processed"},
	})
	if err != nil {
		t.Fatalf("Publish error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&receivedCount) != 1 {
		t.Errorf("Expected 1 event received, got %d", atomic.LoadInt32(&receivedCount))
	}
}
