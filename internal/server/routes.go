package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"nanodb/internal/auth"
	"nanodb/internal/backup"
	"nanodb/internal/database"
	"nanodb/internal/monitor"
	"nanodb/internal/query"
	"nanodb/internal/rbac"
	"nanodb/pkg/api"
)

type Handler struct {
	executor      *database.Executor
	authenticator *auth.Authenticator
	validator     *query.Validator
	systemDB      *database.SystemDB
	backupMgr     *backup.Manager
	monitor       *monitor.Monitor
}

func NewHandler(executor *database.Executor, authenticator *auth.Authenticator, systemDB *database.SystemDB, backupMgr *backup.Manager, mon *monitor.Monitor) *Handler {
	return &Handler{
		executor:      executor,
		authenticator: authenticator,
		validator:     query.NewValidator(),
		systemDB:      systemDB,
		backupMgr:     backupMgr,
		monitor:       mon,
	}
}

func (h *Handler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	var req api.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "invalid request body", Code: 400})
		return
	}

	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "query is required", Code: 400})
		return
	}

	user := auth.UserFromContext(r.Context())

	if blocked, reason := h.validator.CheckDangerous(req.Query); blocked {
		h.logAudit(user, r, req.Query, "dangerous_query", "blocked")
		writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: reason, Code: 403})
		return
	}

	qt := h.validator.IdentifyType(req.Query)
	perm, ok := h.validator.RequiredPermission(qt)
	if ok && user != nil {
		role := rbac.Role(user.Role)
		if !rbac.HasPermission(role, perm) {
			h.logAudit(user, r, req.Query, "permission_denied", "blocked")
			writeJSON(w, http.StatusForbidden, api.ErrorResponse{
				Error: "role '" + user.Role + "' does not have permission for " + string(qt) + " queries",
				Code:  403,
			})
			return
		}
	}

	dbName := req.Database
	if dbName == "" {
		dbName = "main"
	}

	res, err := h.executor.Execute(dbName, req.Query)
	if err != nil {
		h.logAudit(user, r, req.Query, "query_error", "failed")
		writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error(), Code: 500})
		return
	}

	h.logAudit(user, r, req.Query, "/query", "success")
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) HandleTransaction(w http.ResponseWriter, r *http.Request) {
	var req api.TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "invalid request body", Code: 400})
		return
	}

	if len(req.Queries) == 0 {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "queries are required", Code: 400})
		return
	}

	user := auth.UserFromContext(r.Context())

	for _, q := range req.Queries {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}

		if blocked, reason := h.validator.CheckDangerous(q); blocked {
			h.logAudit(user, r, q, "dangerous_query_tx", "blocked")
			writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: reason, Code: 403})
			return
		}

		qt := h.validator.IdentifyType(q)
		perm, ok := h.validator.RequiredPermission(qt)
		if ok && user != nil {
			role := rbac.Role(user.Role)
			if !rbac.HasPermission(role, perm) {
				h.logAudit(user, r, q, "permission_denied_tx", "blocked")
				writeJSON(w, http.StatusForbidden, api.ErrorResponse{
					Error: "role '" + user.Role + "' does not have permission for " + string(qt) + " queries",
					Code:  403,
				})
				return
			}
		}
	}

	dbName := req.Database
	if dbName == "" {
		dbName = "main"
	}

	res, err := h.executor.ExecuteTransaction(dbName, req.Queries)
	if err != nil {
		h.logAudit(user, r, strings.Join(req.Queries, "; "), "tx_error", "failed")
		writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error(), Code: 500})
		return
	}

	h.logAudit(user, r, strings.Join(req.Queries, "; "), "/transaction", "success")
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "invalid request body", Code: 400})
		return
	}

	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "username and password are required", Code: 400})
		return
	}

	ip := r.RemoteAddr
	res, err := h.authenticator.Login(req, ip)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, api.ErrorResponse{Error: err.Error(), Code: 401})
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil || !rbac.HasPermission(rbac.Role(user.Role), rbac.PermCreateUser) {
		writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: "only admins can create users", Code: 403})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "invalid request body", Code: 400})
		return
	}

	if req.Username == "" || req.Password == "" || req.Role == "" {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "username, password, and role are required", Code: 400})
		return
	}

	if _, ok := rbac.ParseRole(req.Role); !ok {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "invalid role; must be one of: admin, developer, readonly, auditor", Code: 400})
		return
	}

	created, err := h.authenticator.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error(), Code: 500})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":       created.ID,
		"username": created.Username,
		"role":     created.Role,
	})
}

func (h *Handler) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil || !rbac.HasPermission(rbac.Role(user.Role), rbac.PermCreateUser) {
		writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: "only admins can list users", Code: 403})
		return
	}

	users, err := h.systemDB.ListUsers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error(), Code: 500})
		return
	}

	type userView struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		Role      string `json:"role"`
	}
	view := make([]userView, 0, len(users))
	for _, u := range users {
		view = append(view, userView{ID: u.ID, Username: u.Username, Role: u.Role})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"users": view})
}

func (h *Handler) HandleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil || !rbac.HasPermission(rbac.Role(user.Role), rbac.PermBackup) {
		writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: "only admins can create API keys", Code: 403})
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "invalid request body", Code: 400})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "name is required", Code: 400})
		return
	}

	rawKey, err := h.authenticator.GenerateAPIKey(user.ID, req.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error(), Code: 500})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"api_key": rawKey, "name": req.Name})
}

func (h *Handler) HandleAuditLogs(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil || !rbac.HasPermission(rbac.Role(user.Role), rbac.PermAuditLog) {
		writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: "only admins and auditors can view audit logs", Code: 403})
		return
	}

	logs, err := h.systemDB.GetAuditLogs(100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error(), Code: 500})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"logs": logs})
}

func (h *Handler) HandleBackup(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil || !rbac.HasPermission(rbac.Role(user.Role), rbac.PermBackup) {
		writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: "only admins can perform backups", Code: 403})
		return
	}

	var req struct {
		Database string `json:"database"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Database == "" {
		req.Database = "main"
	}

	info, err := h.backupMgr.CreateBackup(req.Database)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error(), Code: 500})
		return
	}

	h.logAudit(user, r, "backup:"+req.Database, "/backup", "success")
	writeJSON(w, http.StatusCreated, info)
}

func (h *Handler) HandleRestore(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil || !rbac.HasPermission(rbac.Role(user.Role), rbac.PermRestore) {
		writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: "only admins can perform restores", Code: 403})
		return
	}

	var req struct {
		BackupFile string `json:"backup_file"`
		Database   string `json:"database"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "invalid request body", Code: 400})
		return
	}

	if req.BackupFile == "" {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "backup_file is required", Code: 400})
		return
	}
	if req.Database == "" {
		req.Database = "main"
	}

	if err := h.backupMgr.RestoreBackup(req.BackupFile, req.Database); err != nil {
		writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error(), Code: 500})
		return
	}

	h.logAudit(user, r, "restore:"+req.BackupFile, "/restore", "success")
	writeJSON(w, http.StatusOK, map[string]string{"message": "restore completed", "database": req.Database})
}

func (h *Handler) HandleListBackups(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil || !rbac.HasPermission(rbac.Role(user.Role), rbac.PermBackup) {
		writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: "only admins can list backups", Code: 403})
		return
	}

	backups, err := h.backupMgr.ListBackups()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: err.Error(), Code: 500})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"backups": backups})
}

func (h *Handler) HandleStats(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil || !rbac.HasPermission(rbac.Role(user.Role), rbac.PermAuditLog) {
		writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: "only admins and auditors can view stats", Code: 403})
		return
	}

	stats := h.monitor.Stats()
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) HandlePrometheus(w http.ResponseWriter, r *http.Request) {
	stats := h.monitor.Stats()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "# HELP sparkdb_uptime_seconds Server uptime\n")
	fmt.Fprintf(w, "# TYPE sparkdb_uptime_seconds gauge\n")
	fmt.Fprintf(w, "sparkdb_uptime_seconds %f\n", stats.UptimeSeconds)
	fmt.Fprintf(w, "# HELP sparkdb_queries_total Total queries executed\n")
	fmt.Fprintf(w, "# TYPE sparkdb_queries_total counter\n")
	fmt.Fprintf(w, "sparkdb_queries_total %d\n", stats.TotalQueries)
	fmt.Fprintf(w, "# HELP sparkdb_failed_logins_total Total failed login attempts\n")
	fmt.Fprintf(w, "# TYPE sparkdb_failed_logins_total counter\n")
	fmt.Fprintf(w, "sparkdb_failed_logins_total %d\n", stats.FailedLogins)
	fmt.Fprintf(w, "# HELP sparkdb_active_connections Current active connections\n")
	fmt.Fprintf(w, "# TYPE sparkdb_active_connections gauge\n")
	fmt.Fprintf(w, "sparkdb_active_connections %d\n", stats.ActiveConns)
	fmt.Fprintf(w, "# HELP sparkdb_query_latency_ms Query latency in ms\n")
	fmt.Fprintf(w, "# TYPE sparkdb_query_latency_ms gauge\n")
	fmt.Fprintf(w, "sparkdb_query_latency_ms %f\n", stats.AvgLatencyMs)
	fmt.Fprintf(w, "# HELP sparkdb_goroutines Number of goroutines\n")
	fmt.Fprintf(w, "# TYPE sparkdb_goroutines gauge\n")
	fmt.Fprintf(w, "sparkdb_goroutines %d\n", stats.Goroutines)
	fmt.Fprintf(w, "# HELP sparkdb_memory_alloc_mb Allocated memory\n")
	fmt.Fprintf(w, "# TYPE sparkdb_memory_alloc_mb gauge\n")
	fmt.Fprintf(w, "sparkdb_memory_alloc_mb %f\n", stats.AllocMB)
	for _, db := range stats.Databases {
		fmt.Fprintf(w, "sparkdb_database_size_bytes{database=\"%s\"} %d\n", db.Name, db.Size)
	}
}

func (h *Handler) logAudit(user *auth.AuthUser, r *http.Request, query, endpoint, status string) {
	if h.systemDB == nil {
		return
	}
	username := "anonymous"
	if user != nil {
		username = user.Username
	}
	userID := (*int64)(nil)
	if user != nil {
		userID = &user.ID
	}
	h.systemDB.LogAudit(userID, username, r.RemoteAddr, query, endpoint, status)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
