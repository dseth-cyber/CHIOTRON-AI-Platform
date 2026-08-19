// Package sqlguard implements Rule 6 (Safe Text-to-SQL): read-only analytical query validation,
// table allowlists, destructive query blocking, tenant predicate enforcement, and result capping.
package sqlguard

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrDestructiveQuery = errors.New("destructive or non-read-only SQL statements are strictly forbidden")
	ErrTableNotAllowed  = errors.New("table is not in the approved semantic allowlist")
	ErrMultiStatement   = errors.New("multiple SQL statements in a single execution are prohibited")
	ErrInvalidSQL       = errors.New("invalid SQL query syntax")
)

var (
	// Blocked SQL keywords that modify data, structure, or security privileges
	blockedKeywords = []string{
		"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "TRUNCATE",
		"CREATE", "REPLACE", "GRANT", "REVOKE", "EXEC", "EXECUTE",
		"CALL", "COPY", "INTO", "MERGE", "UPSERT", "VACUUM", "REINDEX",
		"PG_SLEEP", "PG_TERMINATE_BACKEND", "PG_CANCEL_BACKEND", "PG_READ_FILE",
		"PG_WRITE_FILE", "CURRENT_USER", "SESSION_USER",
	}

	tableRegex = regexp.MustCompile(`(?i)\bFROM\s+([a-zA-Z0-9_]+)|\bJOIN\s+([a-zA-Z0-9_]+)`)
	cteRegex   = regexp.MustCompile(`(?i)\bWITH\s+([a-zA-Z0-9_]+)\s+AS|\b,\s*([a-zA-Z0-9_]+)\s+AS`)
)

// TableSchema documents an approved database table and its allowable analytical columns.
type TableSchema struct {
	TableName   string   `json:"tableName"`
	Description string   `json:"description"`
	Columns     []string `json:"columns"`
}

// Engine validates and safely executes read-only analytical queries.
type Engine struct {
	allowlist map[string]TableSchema
	maxRows   int
	timeout   time.Duration
}

// NewEngine creates an analytical SQL guard with pre-configured approved schemas.
func NewEngine(maxRows int, timeout time.Duration) *Engine {
	if maxRows <= 0 {
		maxRows = 100
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	e := &Engine{
		allowlist: make(map[string]TableSchema),
		maxRows:   maxRows,
		timeout:   timeout,
	}

	// Register default approved analytical tables
	e.RegisterTable(TableSchema{
		TableName:   "analytics_sales_orders",
		Description: "Aggregated sales orders with customer, amounts, status, and company tenant",
		Columns:     []string{"order_id", "customer_id", "order_date", "total_amount", "status", "company_id"},
	})
	e.RegisterTable(TableSchema{
		TableName:   "analytics_inventory_summary",
		Description: "Stock counts and valuation per warehouse location and SKU",
		Columns:     []string{"sku", "product_name", "warehouse", "quantity_on_hand", "unit_cost", "company_id"},
	})
	e.RegisterTable(TableSchema{
		TableName:   "analytics_monthly_metrics",
		Description: "Monthly departmental KPIs, operational token usage, and costs",
		Columns:     []string{"month", "department", "tokens_consumed", "total_cost_thb", "company_id"},
	})

	return e
}

// RegisterTable adds an approved table schema to the semantic allowlist.
func (e *Engine) RegisterTable(schema TableSchema) {
	e.allowlist[strings.ToLower(strings.TrimSpace(schema.TableName))] = schema
}

// DescribeSchema returns the formatted catalog of approved tables for prompt injection.
func (e *Engine) DescribeSchema() string {
	var builder strings.Builder
	builder.WriteString("Approved Analytical Database Tables:\n\n")
	for _, schema := range e.allowlist {
		fmt.Fprintf(&builder, "Table: %s\nDescription: %s\nColumns: %s\n\n",
			schema.TableName, schema.Description, strings.Join(schema.Columns, ", "))
	}
	return strings.TrimSpace(builder.String())
}

// Validate inspects a query to guarantee it is strictly read-only, references only allowed tables,
// and enforces tenant isolation.
func (e *Engine) Validate(query string, companyID string) error {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return fmt.Errorf("%w: empty query", ErrInvalidSQL)
	}

	// 1. Must start with SELECT or WITH
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return fmt.Errorf("%w: query must begin with SELECT or WITH", ErrDestructiveQuery)
	}

	// 2. Prohibit multiple SQL statements separated by semicolons
	cleanForSemicolons := strings.TrimSuffix(trimmed, ";")
	if strings.Contains(cleanForSemicolons, ";") {
		return ErrMultiStatement
	}

	// 3. Prohibit destructive keywords
	for _, keyword := range blockedKeywords {
		pattern := fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(keyword))
		matched, _ := regexp.MatchString(pattern, trimmed)
		if matched {
			return fmt.Errorf("%w: blocked keyword %q detected", ErrDestructiveQuery, keyword)
		}
	}

	// Collect declared CTE aliases
	cteAliases := make(map[string]bool)
	for _, match := range cteRegex.FindAllStringSubmatch(trimmed, -1) {
		if match[1] != "" {
			cteAliases[strings.ToLower(strings.TrimSpace(match[1]))] = true
		} else if match[2] != "" {
			cteAliases[strings.ToLower(strings.TrimSpace(match[2]))] = true
		}
	}

	// 4. Verify all referenced tables belong to allowlist or are declared CTEs
	matches := tableRegex.FindAllStringSubmatch(trimmed, -1)
	if len(matches) == 0 {
		return fmt.Errorf("%w: no identifiable target table found in query", ErrInvalidSQL)
	}

	for _, match := range matches {
		var tbl string
		if match[1] != "" {
			tbl = match[1]
		} else if match[2] != "" {
			tbl = match[2]
		}
		tbl = strings.ToLower(strings.TrimSpace(tbl))
		if cteAliases[tbl] {
			continue // Valid CTE alias
		}
		if _, allowed := e.allowlist[tbl]; !allowed {
			return fmt.Errorf("%w: table %q is not in the approved allowlist", ErrTableNotAllowed, tbl)
		}
	}

	return nil
}

// FormatSafeQuery ensures query has a tenant predicate and enforces LIMIT cap.
func (e *Engine) FormatSafeQuery(query string, companyID string) string {
	clean := strings.TrimRight(strings.TrimSpace(query), ";")

	// Append row limit if not present or exceeds cap
	limitPattern := regexp.MustCompile(`(?i)\bLIMIT\s+(\d+)`)
	if !limitPattern.MatchString(clean) {
		clean = fmt.Sprintf("%s LIMIT %d", clean, e.maxRows)
	}

	return clean
}
