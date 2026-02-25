package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"nanodb/internal/monitor"
	"nanodb/pkg/api"
)

type Executor struct {
	manager *Manager
	mon     *monitor.Monitor
}

func NewExecutor(manager *Manager) *Executor {
	return &Executor{manager: manager}
}

func NewExecutorWithMonitor(manager *Manager, mon *monitor.Monitor) *Executor {
	return &Executor{manager: manager, mon: mon}
}

func (e *Executor) Execute(dbName, query string) (*api.QueryResponse, error) {
	start := time.Now()

	db, err := e.manager.Open(dbName)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	q := strings.TrimSpace(query)
	if q == "" {
		return &api.QueryResponse{Error: "empty query"}, nil
	}

	upper := strings.ToUpper(q)
	var res *api.QueryResponse
	if strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "PRAGMA") ||
		strings.HasPrefix(upper, "EXPLAIN") {
		res, err = e.executeQuery(db, q, start)
	} else {
		res, err = e.executeExec(db, q, start)
	}

	if e.mon != nil {
		e.mon.RecordQuery(time.Since(start))
	}
	return res, err
}

func (e *Executor) ExecuteTransaction(dbName string, queries []string) (*api.TransactionResponse, error) {
	db, err := e.manager.Open(dbName)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	results := make([]api.QueryResponse, 0, len(queries))
	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}

		start := time.Now()
		upper := strings.ToUpper(q)

		var res *api.QueryResponse
		if strings.HasPrefix(upper, "SELECT") ||
			strings.HasPrefix(upper, "PRAGMA") ||
			strings.HasPrefix(upper, "EXPLAIN") {
			res, err = e.executeQueryTx(tx, q, start)
		} else {
			res, err = e.executeExecTx(tx, q, start)
		}
		if err != nil {
			return &api.TransactionResponse{
				Results: results,
				Error:   fmt.Sprintf("query failed: %v", err),
			}, nil
		}
		results = append(results, *res)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}
	return &api.TransactionResponse{Results: results}, nil
}

func (e *Executor) executeQuery(db *sql.DB, query string, start time.Time) (*api.QueryResponse, error) {
	rows, err := db.Query(query)
	if err != nil {
		return &api.QueryResponse{Error: err.Error()}, nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return &api.QueryResponse{Error: err.Error()}, nil
	}

	var allRows [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			return &api.QueryResponse{Error: err.Error()}, nil
		}
		allRows = append(allRows, values)
	}

	return &api.QueryResponse{
		Columns: columns,
		Rows:    allRows,
		Time:    time.Since(start).String(),
	}, nil
}

func (e *Executor) executeExec(db *sql.DB, query string, start time.Time) (*api.QueryResponse, error) {
	result, err := db.Exec(query)
	if err != nil {
		return &api.QueryResponse{Error: err.Error()}, nil
	}

	lastID, _ := result.LastInsertId()
	rowsAff, _ := result.RowsAffected()

	return &api.QueryResponse{
		Columns: []string{"last_insert_id", "rows_affected"},
		Rows:    [][]interface{}{{lastID, rowsAff}},
		Time:    time.Since(start).String(),
	}, nil
}

func (e *Executor) executeQueryTx(tx *sql.Tx, query string, start time.Time) (*api.QueryResponse, error) {
	rows, err := tx.Query(query)
	if err != nil {
		return &api.QueryResponse{Error: err.Error()}, nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return &api.QueryResponse{Error: err.Error()}, nil
	}

	var allRows [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			return &api.QueryResponse{Error: err.Error()}, nil
		}
		allRows = append(allRows, values)
	}

	return &api.QueryResponse{
		Columns: columns,
		Rows:    allRows,
		Time:    time.Since(start).String(),
	}, nil
}

func (e *Executor) executeExecTx(tx *sql.Tx, query string, start time.Time) (*api.QueryResponse, error) {
	result, err := tx.Exec(query)
	if err != nil {
		return &api.QueryResponse{Error: err.Error()}, nil
	}

	lastID, _ := result.LastInsertId()
	rowsAff, _ := result.RowsAffected()

	return &api.QueryResponse{
		Columns: []string{"last_insert_id", "rows_affected"},
		Rows:    [][]interface{}{{lastID, rowsAff}},
		Time:    time.Since(start).String(),
	}, nil
}
