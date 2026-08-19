package sqlguard

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSQLGuardValidation(t *testing.T) {
	engine := NewEngine(50, 5*time.Second)

	// 1. Valid Analytical Queries
	validQueries := []string{
		"SELECT order_id, total_amount FROM analytics_sales_orders WHERE status = 'fulfilled'",
		"SELECT warehouse, sum(quantity_on_hand) FROM analytics_inventory_summary GROUP BY warehouse",
		"WITH monthly AS (SELECT month, tokens_consumed FROM analytics_monthly_metrics) SELECT * FROM monthly",
	}
	for _, q := range validQueries {
		if err := engine.Validate(q, "acme"); err != nil {
			t.Errorf("Validate(%q) returned error: %v, want nil", q, err)
		}
	}

	// 2. Prohibited Destructive Queries
	destructiveQueries := []string{
		"DROP TABLE analytics_sales_orders",
		"DELETE FROM analytics_sales_orders WHERE order_id = '1'",
		"UPDATE analytics_inventory_summary SET quantity_on_hand = 0",
		"INSERT INTO analytics_sales_orders VALUES ('1', '2', '2026-01-01', 100, 'open', 'acme')",
		"TRUNCATE analytics_monthly_metrics",
		"ALTER TABLE analytics_sales_orders ADD COLUMN secret text",
		"SELECT * FROM analytics_sales_orders; DROP TABLE analytics_sales_orders",
		"SELECT pg_sleep(10)",
	}
	for _, q := range destructiveQueries {
		if err := engine.Validate(q, "acme"); err == nil {
			t.Errorf("Validate(%q) succeeded, want rejection", q)
		}
	}

	// 3. Unapproved Table Query
	unapprovedQuery := "SELECT * FROM users"
	if err := engine.Validate(unapprovedQuery, "acme"); !errors.Is(err, ErrTableNotAllowed) {
		t.Errorf("Validate(%q) = %v, want ErrTableNotAllowed", unapprovedQuery, err)
	}

	// 4. Multiple Statements
	multiStmt := "SELECT 1 FROM analytics_sales_orders; SELECT 2 FROM analytics_sales_orders"
	if err := engine.Validate(multiStmt, "acme"); !errors.Is(err, ErrMultiStatement) {
		t.Errorf("Validate multi statement = %v, want ErrMultiStatement", err)
	}
}

func TestSQLGuardFormatSafeQuery(t *testing.T) {
	engine := NewEngine(50, 5*time.Second)

	q := "SELECT order_id FROM analytics_sales_orders"
	formatted := engine.FormatSafeQuery(q, "acme")

	if !strings.Contains(formatted, "LIMIT 50") {
		t.Errorf("FormatSafeQuery() = %q, want LIMIT 50 appended", formatted)
	}
}

func TestDescribeSchema(t *testing.T) {
	engine := NewEngine(100, 10*time.Second)
	desc := engine.DescribeSchema()

	if !strings.Contains(desc, "analytics_sales_orders") ||
		!strings.Contains(desc, "analytics_inventory_summary") {
		t.Errorf("DescribeSchema() missing expected tables:\n%s", desc)
	}
}
