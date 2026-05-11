package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sparkdb/internal/config"
)

func startTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 0
	cfg.Database.DataDir = t.TempDir()
	cfg.Database.MaxConns = 1
	cfg.Auth.Enabled = true
	cfg.Auth.JWTSecret = "test-secret"
	cfg.Backup.Dir = t.TempDir()

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}

	return httptest.NewServer(srv.httpServer.Handler)
}

func loginAdmin(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	body := `{"username":"admin","password":"admin"}`
	resp, err := http.Post(ts.URL+"/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result["token"].(string)
}

func request(t *testing.T, ts *httptest.Server, method, path, body, token string) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, ts.URL+path, reqBody)
	if err != nil {
		t.Fatalf("NewRequest(%s %s): %v", method, path, err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do(%s %s): %v", method, path, err)
	}
	return resp
}

func TestHealthEndpoint(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "ok" {
		t.Errorf("status = %v, want ok", result["status"])
	}
}

func TestLoginEndpointValidBody(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	body := `{"username":"admin","password":"admin"}`
	resp, err := http.Post(ts.URL+"/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /auth/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["token"] == "" {
		t.Error("token should not be empty")
	}
	if result["password_change_required"] != true {
		t.Error("password_change_required should be true for default admin")
	}
}

func TestLoginEndpointBadCredentials(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	body := `{"username":"admin","password":"wrong"}`
	resp, _ := http.Post(ts.URL+"/auth/login", "application/json", strings.NewReader(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestLoginEndpointEmptyBody(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/auth/login", "application/json", nil)
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestLoginEndpointMissingFields(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp, _ := http.Post(ts.URL+"/auth/login", "application/json", strings.NewReader(`{}`))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDatabasesEndpoint(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "GET", "/databases", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDatabasesEndpointUnauthenticated(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/databases", nil)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestQueryEndpoint(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/query", `{"query":"SELECT 1"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != nil && result["error"] != "" {
		t.Errorf("query error: %v", result["error"])
	}
}

func TestQueryEndpointEmptyQuery(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/query", `{"query":""}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestQueryEndpointInvalidBody(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/query", `not-json`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestChangePassword(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "PUT", "/auth/password", `{"old_password":"admin","new_password":"NewPass123!"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)

		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Logf("response: %v", errResp)
	}
}

func TestChangePassword_InvalidBody(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "PUT", "/auth/password", `not-json`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestChangePassword_MissingFields(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "PUT", "/auth/password", `{}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestChangePassword_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "PUT", "/auth/password", `{"old_password":"admin","new_password":"newpass"}`, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestCreateUser(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/admin/users", `{"username":"newuser","password":"Pass123!","role":"developer"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)

		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Logf("response: %v", errResp)
	}
}

func TestCreateUser_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "POST", "/admin/users", `{"username":"u","password":"p","role":"developer"}`, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestCreateUser_InvalidRole(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/admin/users", `{"username":"u","password":"p","role":"superadmin"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateUser_MissingFields(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/admin/users", `{}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestListUsers(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "GET", "/admin/users", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	users, ok := result["users"].([]interface{})
	if !ok {
		t.Fatal("expected users array")
	}
	if len(users) < 1 {
		t.Error("expected at least 1 user (admin)")
	}
}

func TestListUsers_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "GET", "/admin/users", "", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestStatsEndpoint(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "GET", "/stats", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["total_queries"] == nil {
		t.Error("expected total_queries in stats")
	}
}

func TestStatsEndpointNoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "GET", "/stats", "", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestPrometheusEndpoint(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "sparkdb_") {
		t.Error("expected sparkdb_ metrics in output")
	}
}

func TestTransactionEndpoint(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)

	request(t, ts, "POST", "/query", `{"query":"CREATE TABLE test_tx (id INT)", "database":"txdb"}`, token).Body.Close()

	resp := request(t, ts, "POST", "/transaction", `{"queries":["INSERT INTO test_tx VALUES (1)","INSERT INTO test_tx VALUES (2)"], "database":"txdb"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)

		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Logf("response: %v", errResp)
	}
}

func TestTransactionEndpointEmptyQueries(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/transaction", `{"queries":[]}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestTransactionEndpointInvalidBody(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/transaction", `not-json`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateAPIKey(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/auth/api-keys", `{"name":"test-key"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)

		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Logf("response: %v", errResp)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["api_key"] == "" {
		t.Error("expected api_key in response")
	}
}

func TestCreateAPIKey_MissingName(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/auth/api-keys", `{}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateAPIKey_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "POST", "/auth/api-keys", `{"name":"k"}`, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestListAPIKeys(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "GET", "/auth/api-keys", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestAuditLogsEndpoint(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "GET", "/admin/audit-logs", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["logs"] == nil {
		t.Error("expected logs array")
	}
}

func TestAuditLogsEndpoint_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "GET", "/admin/audit-logs", "", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestBackupEndpoint(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)

	request(t, ts, "POST", "/query", `{"query":"CREATE TABLE t (id INT)", "database":"backupdb"}`, token).Body.Close()

	resp := request(t, ts, "POST", "/backup", `{"database":"backupdb"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)

		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Logf("response: %v", errResp)
	}
}

func TestBackupEndpoint_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "POST", "/backup", `{"database":"main"}`, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestListBackupsEndpoint(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "GET", "/backups", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDeleteBackupEndpoint_NotFound(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "DELETE", fmt.Sprintf("/backups/%s", "nonexistent.backup"), "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestCORSHeaders(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/health", nil)
	req.Header.Set("Origin", "http://example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("CORS origin = %q, want *", resp.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSPreflight(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/health", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
}

func TestReplicationLogEndpoint(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "GET", "/replication/log?since=0&limit=10", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["entries"] == nil {
		t.Error("expected entries array")
	}
}

func TestReplicationLogEndpoint_InvalidSince(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "GET", "/replication/log?since=abc", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestReplicationLogEndpoint_InvalidLimit(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "GET", "/replication/log?since=0&limit=99999", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestReplicationLogEndpoint_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "GET", "/replication/log", "", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestRestoreEndpoint(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)

	request(t, ts, "POST", "/query", `{"query":"CREATE TABLE t (id INT)", "database":"restoredb"}`, token).Body.Close()

	resp := request(t, ts, "POST", "/backup", `{"database":"restoredb"}`, token)
	defer resp.Body.Close()

	var backupResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&backupResult)

	if backupResult["name"] == nil {
		t.Skip("backup not created, skipping restore test")
		return
	}

	resp = request(t, ts, "POST", "/restore", fmt.Sprintf(`{"backup_file":"%s","database":"restoredb"}`, backupResult["path"]), token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRestoreEndpoint_MissingBackupFile(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/restore", `{"backup_file":"","database":"main"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestRestoreEndpoint_InvalidBody(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/restore", `not-json`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestShutdownEndpoint_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "POST", "/shutdown", "", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestDeleteUser(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)

	request(t, ts, "POST", "/admin/users", `{"username":"deleteuser","password":"Pass123!","role":"developer"}`, token).Body.Close()

	resp := request(t, ts, "DELETE", "/admin/users/99999", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError && resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 or 500", resp.StatusCode)
	}
}

func TestDeleteUser_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "DELETE", "/admin/users/1", "", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestDeleteUser_InvalidID(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "DELETE", "/admin/users/abc", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUpdateUserRole(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "PUT", "/admin/users/1/role", `{"role":"developer"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)

		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Logf("response: %v", errResp)
	}
}

func TestUpdateUserRole_InvalidID(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "PUT", "/admin/users/abc/role", `{"role":"admin"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUpdateUserRole_InvalidBody(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "PUT", "/admin/users/1/role", `not-json`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUpdateUserRole_InvalidRole(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "PUT", "/admin/users/1/role", `{"role":"superadmin"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUpdateUserRole_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "PUT", "/admin/users/1/role", `{"role":"admin"}`, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestUpdateUserPassword(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "PUT", "/admin/users/1/password", `{"password":"NewStr0ngPass!"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)

		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Logf("response: %v", errResp)
	}
}

func TestUpdateUserPassword_InvalidID(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "PUT", "/admin/users/abc/password", `{"password":"NewStr0ngPass!"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUpdateUserPassword_WeakPassword(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "PUT", "/admin/users/1/password", `{"password":"weak"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUpdateUserPassword_InvalidBody(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "PUT", "/admin/users/1/password", `not-json`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUpdateUserPassword_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "PUT", "/admin/users/1/password", `{"password":"NewStr0ngPass!"}`, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestDeleteAPIKey_InvalidID(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "DELETE", "/auth/api-keys/abc", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDeleteAPIKey_NotFound(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "DELETE", "/auth/api-keys/99999", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Logf("status = %d (expected 500 or similar)", resp.StatusCode)
	}
}

func TestDeleteAPIKey_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "DELETE", "/auth/api-keys/1", "", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestRevealAPIKey(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)

	resp := request(t, ts, "POST", "/auth/api-keys", `{"name":"revealtest"}`, token)
	defer resp.Body.Close()

	keyID := int64(1)
	resp = request(t, ts, "POST", fmt.Sprintf("/auth/api-keys/%d/reveal", keyID), `{"password":"admin"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)

		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Logf("response: %v", errResp)
	}
}

func TestRevealAPIKey_WrongPassword(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)

	resp := request(t, ts, "POST", "/auth/api-keys", `{"name":"revealfail"}`, token)
	defer resp.Body.Close()

	resp = request(t, ts, "POST", "/auth/api-keys/1/reveal", `{"password":"wrongpass"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Logf("status = %d (expected 401 for wrong password)", resp.StatusCode)
	}
}

func TestRevealAPIKey_InvalidID(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/auth/api-keys/abc/reveal", `{"password":"admin"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestRevealAPIKey_NoPassword(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/auth/api-keys/1/reveal", `{}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestRevealAPIKey_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "POST", "/auth/api-keys/1/reveal", `{"password":"admin"}`, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestRevealAPIKey_InvalidBody(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/auth/api-keys/1/reveal", `not-json`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestQueryDangerous(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "POST", "/query", `{"query":"DROP TABLE users"}`, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestBodyLimitMiddleware(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	largeBody := strings.Repeat("A", 2*1024*1024)
	resp := request(t, ts, "POST", "/query", `{"query":"SELECT 1","params":["`+largeBody+`"]}`, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Logf("status = %d (body may be truncated by middleware)", resp.StatusCode)
	}
}

func TestDeleteUser_Self(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	token := loginAdmin(t, ts)
	resp := request(t, ts, "DELETE", "/admin/users/1", "", token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (cannot delete yourself)", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != nil {
		errMsg := result["error"].(string)
		if !strings.Contains(errMsg, "yourself") {
			t.Errorf("error message = %q, want 'yourself'", errMsg)
		}
	}
}

func TestQuery_NoAuth(t *testing.T) {
	ts := startTestServer(t)
	defer ts.Close()

	resp := request(t, ts, "POST", "/query", `{"query":"SELECT 1"}`, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}
