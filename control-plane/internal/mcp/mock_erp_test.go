package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMockERPToolsAndCall(t *testing.T) {
	erp := NewMockERP("erp")
	if erp.Slug() != "erp" {
		t.Errorf("Slug() = %q, want 'erp'", erp.Slug())
	}

	tools := erp.Tools()
	if len(tools) != 3 {
		t.Fatalf("Tools() returned %d tools, want 3", len(tools))
	}

	ctx := context.Background()

	// 1. Test Inventory Lookup (Acme)
	res, err := erp.Call(ctx, "erp.inventory.lookup", map[string]any{
		"query":      "valve",
		"company_id": "acme",
	})
	if err != nil {
		t.Fatalf("Call inventory lookup error: %v", err)
	}
	if res.IsError {
		t.Fatalf("Call returned error: %s", res.Text())
	}

	var items []ERPInventoryItem
	if err := json.Unmarshal([]byte(res.Text()), &items); err != nil {
		t.Fatalf("Unmarshal inventory: %v", err)
	}
	if len(items) != 1 || items[0].SKU != "SKU-1001" {
		t.Errorf("Unexpected inventory items: %+v", items)
	}

	// 2. Test Tenant Isolation (Globex SKU should not show for Acme)
	resGlobex, err := erp.Call(ctx, "erp.inventory.lookup", map[string]any{
		"query":      "turbine",
		"company_id": "acme",
	})
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}
	var globexItems []ERPInventoryItem
	_ = json.Unmarshal([]byte(resGlobex.Text()), &globexItems)
	if len(globexItems) != 0 {
		t.Errorf("Acme was able to see Globex inventory: %+v", globexItems)
	}

	// 3. Test Order Status
	orderRes, err := erp.Call(ctx, "erp.order.status", map[string]any{
		"order_id":   "PO-2026-001",
		"company_id": "acme",
	})
	if err != nil {
		t.Fatalf("Call order status error: %v", err)
	}
	if orderRes.IsError {
		t.Fatalf("Call returned error: %s", orderRes.Text())
	}

	var order ERPOrder
	if err := json.Unmarshal([]byte(orderRes.Text()), &order); err != nil {
		t.Fatalf("Unmarshal order: %v", err)
	}
	if order.Status != "fulfilled" || order.Total != 70000.0 {
		t.Errorf("Unexpected order: %+v", order)
	}

	// 4. Test Customer Profile
	custRes, err := erp.Call(ctx, "erp.customer.profile", map[string]any{
		"customer_id": "CUST-ACME-01",
		"company_id":  "acme",
	})
	if err != nil {
		t.Fatalf("Call customer profile error: %v", err)
	}
	var cust ERPCustomer
	if err := json.Unmarshal([]byte(custRes.Text()), &cust); err != nil {
		t.Fatalf("Unmarshal customer: %v", err)
	}
	if cust.Tier != "platinum" {
		t.Errorf("Unexpected customer tier: %s", cust.Tier)
	}
}

func TestMockERPErrors(t *testing.T) {
	erp := NewMockERP("erp")
	ctx := context.Background()

	// Missing query
	res, _ := erp.Call(ctx, "erp.inventory.lookup", map[string]any{})
	if !res.IsError {
		t.Errorf("Expected error on missing query")
	}

	// Non-existent order
	res, _ = erp.Call(ctx, "erp.order.status", map[string]any{"order_id": "UNKNOWN-999"})
	if !res.IsError {
		t.Errorf("Expected error on unknown order")
	}

	// Unknown tool
	res, _ = erp.Call(ctx, "unknown.tool", map[string]any{})
	if !res.IsError {
		t.Errorf("Expected error on unknown tool")
	}
}
