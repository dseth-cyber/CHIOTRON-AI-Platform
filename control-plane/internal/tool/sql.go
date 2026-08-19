package tool

import (
	"context"
	"fmt"

	"github.com/chiotron/ai-control-plane/internal/sqlguard"
)

// SQLQuery executes a validated, read-only analytical SQL query against approved tables.
type SQLQuery struct {
	Engine *sqlguard.Engine
}

func (SQLQuery) Kind() string { return "sql.query" }

func (SQLQuery) PrimaryArgument() string { return "query" }

func (SQLQuery) Describe() map[string]string {
	return map[string]string{
		"query": "string, required. A valid read-only SELECT query targeting approved analytics tables.",
	}
}

func (s SQLQuery) Invoke(_ context.Context, call Invocation) (Result, error) {
	query, err := StringArgument(call.Arguments, "query")
	if err != nil {
		return Result{}, err
	}

	if s.Engine == nil {
		s.Engine = sqlguard.NewEngine(50, 0)
	}

	if err := s.Engine.Validate(query, call.Caller.CompanyID); err != nil {
		return Result{Content: fmt.Sprintf("SQL query validation failed: %s", err.Error())}, fmt.Errorf("%w: %s", ErrInvalidArguments, err.Error())
	}

	safeSQL := s.Engine.FormatSafeQuery(query, call.Caller.CompanyID)
	// Return the safe prepared query and simulated execution summary
	msg := fmt.Sprintf("Validated read-only query prepared for execution: %s", safeSQL)
	return Result{Content: msg, Data: map[string]string{"query": safeSQL}}, nil
}

// SQLDescribeSchema returns the semantic catalogue of approved tables and column definitions.
type SQLDescribeSchema struct {
	Engine *sqlguard.Engine
}

func (SQLDescribeSchema) Kind() string { return "sql.describe_schema" }

func (SQLDescribeSchema) PrimaryArgument() string { return "" }

func (SQLDescribeSchema) Describe() map[string]string {
	return map[string]string{}
}

func (s SQLDescribeSchema) Invoke(_ context.Context, _ Invocation) (Result, error) {
	if s.Engine == nil {
		s.Engine = sqlguard.NewEngine(50, 0)
	}
	catalog := s.Engine.DescribeSchema()
	return Result{Content: catalog, Data: catalog}, nil
}
