// Package enterprise implements Phase 10: Authorized ERP API adapters and event-driven AI workflows.
// It complies with Rule 2 (ERP remains system of record) and Rule 8 (Independent failure domains).
package enterprise

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrERPUnavailable = errors.New("erp service is unreachable or returned a temporary failure")
	ErrUnauthorized   = errors.New("unauthorized erp operation or insufficient clearance")
	ErrNotFound       = errors.New("requested erp entity was not found")
)

// CallerContext encapsulates identity and tenant boundary for cross-service header propagation.
type CallerContext struct {
	CompanyID  string
	Department string
	UserID     string
	Clearance  string
}

// InventoryItem models product stock and warehouse records.
type InventoryItem struct {
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Warehouse string  `json:"warehouse"`
	UnitPrice float64 `json:"unitPrice"`
}

// Order models a sales or purchase order in the ERP.
type Order struct {
	ID         string   `json:"id"`
	CustomerID string   `json:"customerId"`
	Total      float64  `json:"total"`
	Status     string   `json:"status"`
	Items      []string `json:"items"`
	Date       string   `json:"date"`
}

// Customer models customer billing and credit limits.
type Customer struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Tier               string  `json:"tier"`
	CreditLimit        float64 `json:"creditLimit"`
	OutstandingBalance float64 `json:"outstandingBalance"`
}

// CreatePOParams specifies fields required to create an authorized purchase order.
type CreatePOParams struct {
	VendorID    string   `json:"vendorId"`
	Items       []string `json:"items"`
	TotalAmount float64  `json:"totalAmount"`
	Description string   `json:"description"`
}

// ApprovalRequestParams defines payload for an approval workflow request.
type ApprovalRequestParams struct {
	WorkflowID  string  `json:"workflowId"`
	Amount      float64 `json:"amount"`
	RequestedBy string  `json:"requestedBy"`
	Reason      string  `json:"reason"`
}

// ApprovalResult represents the workflow outcome.
type ApprovalResult struct {
	RequestID string `json:"requestId"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// ERPAdapter specifies the contract for reading and invoking authorized workflows against ERP.
type ERPAdapter interface {
	GetInventory(ctx context.Context, caller CallerContext, query string) ([]InventoryItem, error)
	GetOrder(ctx context.Context, caller CallerContext, orderID string) (Order, error)
	GetCustomer(ctx context.Context, caller CallerContext, customerID string) (Customer, error)
	CreatePurchaseOrder(ctx context.Context, caller CallerContext, params CreatePOParams) (Order, error)
	SubmitApprovalRequest(ctx context.Context, caller CallerContext, params ApprovalRequestParams) (ApprovalResult, error)
}

// HTTPERPAdapter is an HTTP client adapter that communicates with the upstream ERP gateway.
type HTTPERPAdapter struct {
	baseURL string
	client  *http.Client
}

// NewHTTPERPAdapter creates a new resilient ERP adapter client.
func NewHTTPERPAdapter(baseURL string, timeout time.Duration) *HTTPERPAdapter {
	if baseURL == "" {
		baseURL = "http://erp-gateway.internal"
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPERPAdapter{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{Timeout: timeout},
	}
}

func (a *HTTPERPAdapter) sendRequest(ctx context.Context, caller CallerContext, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal erp request: %w", err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	url := fmt.Sprintf("%s%s", a.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create erp request: %w", err)
	}

	// Propagate Identity & Tenant headers
	req.Header.Set("Content-Type", "application/json")
	if caller.CompanyID != "" {
		req.Header.Set("X-Company-ID", caller.CompanyID)
	}
	if caller.Department != "" {
		req.Header.Set("X-Department", caller.Department)
	}
	if caller.UserID != "" {
		req.Header.Set("X-Caller-ID", caller.UserID)
	}
	if caller.Clearance != "" {
		req.Header.Set("X-Clearance", caller.Clearance)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrERPUnavailable, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read erp response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		return respBody, nil
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	default:
		return nil, fmt.Errorf("%w: erp returned status %d: %s", ErrERPUnavailable, resp.StatusCode, string(respBody))
	}
}

func (a *HTTPERPAdapter) GetInventory(ctx context.Context, caller CallerContext, query string) ([]InventoryItem, error) {
	path := fmt.Sprintf("/api/v1/inventory?query=%s", query)
	data, err := a.sendRequest(ctx, caller, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var items []InventoryItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (a *HTTPERPAdapter) GetOrder(ctx context.Context, caller CallerContext, orderID string) (Order, error) {
	path := fmt.Sprintf("/api/v1/orders/%s", orderID)
	data, err := a.sendRequest(ctx, caller, http.MethodGet, path, nil)
	if err != nil {
		return Order{}, err
	}
	var order Order
	if err := json.Unmarshal(data, &order); err != nil {
		return Order{}, err
	}
	return order, nil
}

func (a *HTTPERPAdapter) GetCustomer(ctx context.Context, caller CallerContext, customerID string) (Customer, error) {
	path := fmt.Sprintf("/api/v1/customers/%s", customerID)
	data, err := a.sendRequest(ctx, caller, http.MethodGet, path, nil)
	if err != nil {
		return Customer{}, err
	}
	var cust Customer
	if err := json.Unmarshal(data, &cust); err != nil {
		return Customer{}, err
	}
	return cust, nil
}

func (a *HTTPERPAdapter) CreatePurchaseOrder(ctx context.Context, caller CallerContext, params CreatePOParams) (Order, error) {
	data, err := a.sendRequest(ctx, caller, http.MethodPost, "/api/v1/orders", params)
	if err != nil {
		return Order{}, err
	}
	var order Order
	if err := json.Unmarshal(data, &order); err != nil {
		return Order{}, err
	}
	return order, nil
}

func (a *HTTPERPAdapter) SubmitApprovalRequest(ctx context.Context, caller CallerContext, params ApprovalRequestParams) (ApprovalResult, error) {
	data, err := a.sendRequest(ctx, caller, http.MethodPost, "/api/v1/workflows/approvals", params)
	if err != nil {
		return ApprovalResult{}, err
	}
	var result ApprovalResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ApprovalResult{}, err
	}
	return result, nil
}
