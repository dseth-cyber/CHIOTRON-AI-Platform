// Package mcp: In-memory governed Mock ERP client and tools adapter.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// ERPInventoryItem represents stock data in the mock ERP.
type ERPInventoryItem struct {
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Warehouse string  `json:"warehouse"`
	UnitPrice float64 `json:"unitPrice"`
	CompanyID string  `json:"companyId"`
}

// ERPOrder represents a sales or purchase order in the mock ERP.
type ERPOrder struct {
	ID         string   `json:"id"`
	CustomerID string   `json:"customerId"`
	Total      float64  `json:"total"`
	Status     string   `json:"status"`
	Items      []string `json:"items"`
	Date       string   `json:"date"`
	CompanyID  string   `json:"companyId"`
}

// ERPCustomer represents a business customer profile in the mock ERP.
type ERPCustomer struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Tier               string  `json:"tier"`
	CreditLimit        float64 `json:"creditLimit"`
	OutstandingBalance float64 `json:"outstandingBalance"`
	CompanyID          string  `json:"companyId"`
}

// MockERP implements the mcp.Caller interface with governed company-isolated mock ERP data.
type MockERP struct {
	mu          sync.RWMutex
	slug        string
	inventories []ERPInventoryItem
	orders      map[string]ERPOrder
	customers   map[string]ERPCustomer
}

// NewMockERP creates a new mock ERP client pre-seeded with sample enterprise data.
func NewMockERP(slug string) *MockERP {
	if slug == "" {
		slug = "erp"
	}

	erp := &MockERP{
		slug:        slug,
		orders:      make(map[string]ERPOrder),
		customers:   make(map[string]ERPCustomer),
		inventories: make([]ERPInventoryItem, 0),
	}

	// Seed Inventory
	erp.inventories = append(erp.inventories,
		ERPInventoryItem{SKU: "SKU-1001", Name: "Industrial Valve A1", Quantity: 150, Warehouse: "WH-BKK-01", UnitPrice: 1200.0, CompanyID: "acme"},
		ERPInventoryItem{SKU: "SKU-1002", Name: "High-Pressure Pipe 5m", Quantity: 45, Warehouse: "WH-BKK-02", UnitPrice: 3500.0, CompanyID: "acme"},
		ERPInventoryItem{SKU: "SKU-2001", Name: "Turbine Blade Set", Quantity: 12, Warehouse: "WH-GLX-01", UnitPrice: 85000.0, CompanyID: "globex"},
	)

	// Seed Orders
	erp.orders["PO-2026-001"] = ERPOrder{
		ID:         "PO-2026-001",
		CustomerID: "CUST-ACME-01",
		Total:      70000.0,
		Status:     "fulfilled",
		Items:      []string{"20x SKU-1002"},
		Date:       "2026-08-10",
		CompanyID:  "acme",
	}
	erp.orders["PO-2026-002"] = ERPOrder{
		ID:         "PO-2026-002",
		CustomerID: "CUST-ACME-02",
		Total:      14400.0,
		Status:     "processing",
		Items:      []string{"12x SKU-1001"},
		Date:       "2026-08-18",
		CompanyID:  "acme",
	}

	// Seed Customers
	erp.customers["CUST-ACME-01"] = ERPCustomer{
		ID:                 "CUST-ACME-01",
		Name:               "Siam Engineering Ltd",
		Tier:               "platinum",
		CreditLimit:        500000.0,
		OutstandingBalance: 70000.0,
		CompanyID:          "acme",
	}

	return erp
}

func (m *MockERP) Slug() string {
	return m.slug
}

func (m *MockERP) Tools() []Tool {
	return []Tool{
		{
			Name:        "erp.inventory.lookup",
			Title:       "ERP Inventory Lookup",
			Description: "Search stock quantity and warehouse location for products in the ERP.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Product SKU or part of product name to search for.",
					},
					"company_id": map[string]any{
						"type":        "string",
						"description": "Tenant company identifier to enforce context isolation.",
					},
				},
				"required": []any{"query"},
			},
		},
		{
			Name:        "erp.order.status",
			Title:       "ERP Order Status",
			Description: "Check order status, items, and total amounts by Order ID.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"order_id": map[string]any{
						"type":        "string",
						"description": "Purchase or Sales Order ID (e.g., PO-2026-001).",
					},
					"company_id": map[string]any{
						"type":        "string",
						"description": "Tenant company identifier.",
					},
				},
				"required": []any{"order_id"},
			},
		},
		{
			Name:        "erp.customer.profile",
			Title:       "ERP Customer Profile",
			Description: "Retrieve customer tier, credit limit and outstanding balances.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"customer_id": map[string]any{
						"type":        "string",
						"description": "Customer ID (e.g., CUST-ACME-01).",
					},
					"company_id": map[string]any{
						"type":        "string",
						"description": "Tenant company identifier.",
					},
				},
				"required": []any{"customer_id"},
			},
		},
	}
}

func (m *MockERP) Call(_ context.Context, name string, arguments map[string]any) (CallResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	companyID, _ := arguments["company_id"].(string)

	switch name {
	case "erp.inventory.lookup":
		query, _ := arguments["query"].(string)
		query = strings.ToLower(strings.TrimSpace(query))
		if query == "" {
			return CallResult{
				IsError: true,
				Content: []Content{{Type: "text", Text: "query argument is required"}},
			}, nil
		}

		var matches []ERPInventoryItem
		for _, item := range m.inventories {
			if companyID != "" && item.CompanyID != companyID {
				continue
			}
			if strings.Contains(strings.ToLower(item.SKU), query) ||
				strings.Contains(strings.ToLower(item.Name), query) {
				matches = append(matches, item)
			}
		}

		raw, err := json.Marshal(matches)
		if err != nil {
			return CallResult{}, err
		}
		return CallResult{
			Content: []Content{{Type: "text", Text: string(raw)}},
		}, nil

	case "erp.order.status":
		orderID, _ := arguments["order_id"].(string)
		orderID = strings.TrimSpace(orderID)
		if orderID == "" {
			return CallResult{
				IsError: true,
				Content: []Content{{Type: "text", Text: "order_id is required"}},
			}, nil
		}

		order, exists := m.orders[orderID]
		if !exists || (companyID != "" && order.CompanyID != companyID) {
			return CallResult{
				IsError: true,
				Content: []Content{{Type: "text", Text: fmt.Sprintf("order %q not found", orderID)}},
			}, nil
		}

		raw, err := json.Marshal(order)
		if err != nil {
			return CallResult{}, err
		}
		return CallResult{
			Content: []Content{{Type: "text", Text: string(raw)}},
		}, nil

	case "erp.customer.profile":
		custID, _ := arguments["customer_id"].(string)
		custID = strings.TrimSpace(custID)
		if custID == "" {
			return CallResult{
				IsError: true,
				Content: []Content{{Type: "text", Text: "customer_id is required"}},
			}, nil
		}

		cust, exists := m.customers[custID]
		if !exists || (companyID != "" && cust.CompanyID != companyID) {
			return CallResult{
				IsError: true,
				Content: []Content{{Type: "text", Text: fmt.Sprintf("customer %q not found", custID)}},
			}, nil
		}

		raw, err := json.Marshal(cust)
		if err != nil {
			return CallResult{}, err
		}
		return CallResult{
			Content: []Content{{Type: "text", Text: string(raw)}},
		}, nil

	default:
		return CallResult{
			IsError: true,
			Content: []Content{{Type: "text", Text: fmt.Sprintf("unknown erp tool %q", name)}},
		}, nil
	}
}
