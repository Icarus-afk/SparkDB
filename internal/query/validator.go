package query

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"nanodb/internal/rbac"
)

type QueryType string

const (
	TypeSelect QueryType = "SELECT"
	TypeInsert QueryType = "INSERT"
	TypeUpdate QueryType = "UPDATE"
	TypeDelete QueryType = "DELETE"
	TypeCreate QueryType = "CREATE"
	TypeDrop   QueryType = "DROP"
	TypeAlter  QueryType = "ALTER"
	TypePragma QueryType = "PRAGMA"
	TypeExplain QueryType = "EXPLAIN"
	TypeOther  QueryType = "OTHER"
)

type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) IdentifyType(query string) QueryType {
	q := strings.TrimSpace(query)
	upper := strings.ToUpper(q)

	switch {
	case strings.HasPrefix(upper, "SELECT"):
		return TypeSelect
	case strings.HasPrefix(upper, "INSERT"):
		return TypeInsert
	case strings.HasPrefix(upper, "UPDATE"):
		return TypeUpdate
	case strings.HasPrefix(upper, "DELETE"):
		return TypeDelete
	case strings.HasPrefix(upper, "CREATE"):
		return TypeCreate
	case strings.HasPrefix(upper, "DROP"):
		return TypeDrop
	case strings.HasPrefix(upper, "ALTER"):
		return TypeAlter
	case strings.HasPrefix(upper, "PRAGMA"):
		return TypePragma
	case strings.HasPrefix(upper, "EXPLAIN"):
		return TypeExplain
	default:
		return TypeOther
	}
}

func (v *Validator) RequiredPermission(qt QueryType) (rbac.Permission, bool) {
	switch qt {
	case TypeSelect, TypePragma, TypeExplain:
		return rbac.PermQuery, true
	case TypeInsert, TypeUpdate:
		return rbac.PermWrite, true
	case TypeCreate:
		return rbac.PermCreate, true
	case TypeAlter:
		return rbac.PermAlter, true
	case TypeDrop:
		return rbac.PermDrop, true
	case TypeDelete:
		return rbac.PermDelete, true
	default:
		return "", false
	}
}

type DangerLevel int

const (
	DangerNone    DangerLevel = 0
	DangerWarning DangerLevel = 1
	DangerBlock   DangerLevel = 2
)

type DangerPattern struct {
	Pattern     string
	Level       DangerLevel
	Description string
}

var dangerPatterns = []DangerPattern{
	{`DROP DATABASE`, DangerBlock, "dropping databases is not allowed"},
	{`DROP TABLE`, DangerBlock, "dropping tables requires admin role"},
	{`DELETE FROM sqlite_master`, DangerBlock, "modifying system tables is not allowed"},
	{`DROP INDEX`, DangerWarning, "dropping indexes requires caution"},
	{`DROP VIEW`, DangerWarning, "dropping views requires caution"},
	{`DROP TRIGGER`, DangerWarning, "dropping triggers requires caution"},
	{`VACUUM`, DangerWarning, "vacuum may lock the database"},
	{`ATTACH DATABASE`, DangerBlock, "attaching databases is not allowed"},
	{`DETACH DATABASE`, DangerBlock, "detaching databases is not allowed"},
}

func (v *Validator) CheckDangerous(query string) (bool, string) {
	upper := strings.ToUpper(strings.TrimSpace(query))
	for _, dp := range dangerPatterns {
		if strings.Contains(upper, dp.Pattern) {
			if dp.Level == DangerBlock {
				return true, dp.Description
			}
		}
	}
	return false, ""
}

type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-r.window)

	entries := r.attempts[key]
	var valid []time.Time
	for _, t := range entries {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= r.limit {
		r.attempts[key] = valid
		return false
	}

	valid = append(valid, now)
	r.attempts[key] = valid
	return true
}

func (r *RateLimiter) Reset(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, key)
}

type QueryMapping struct {
	QueryType   QueryType
	TableName   string
}

func (v *Validator) Analyze(query string) (*QueryMapping, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("empty query")
	}

	qt := v.IdentifyType(q)
	m := &QueryMapping{QueryType: qt}

	upper := strings.ToUpper(q)

	// Extract table name from common patterns
	if qt == TypeDelete || qt == TypeInsert || qt == TypeSelect || qt == TypeUpdate {
		parts := strings.Fields(upper)
		for i, p := range parts {
			if p == "FROM" || p == "INTO" || p == "TABLE" || p == "UPDATE" {
				if i+1 < len(parts) {
					m.TableName = strings.TrimRight(parts[i+1], ";")
				}
				break
			}
		}
	}

	return m, nil
}
