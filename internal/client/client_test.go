package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := New("localhost", 9600)
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.baseURL != "http://localhost:9600" {
		t.Errorf("baseURL = %q, want http://localhost:9600", c.baseURL)
	}
}

func TestClientSetAPIKey(t *testing.T) {
	c := New("localhost", 9600)
	c.SetAPIKey("test-key-123")
	if c.apiKey != "test-key-123" {
		t.Errorf("apiKey = %q, want test-key-123", c.apiKey)
	}
}

func TestClientAuthToken(t *testing.T) {
	c := New("localhost", 9600)
	c.token = "mytoken"

	req, _ := http.NewRequest("GET", "/test", nil)
	c.auth(req)

	if req.Header.Get("Authorization") != "Bearer mytoken" {
		t.Errorf("Authorization = %q, want Bearer mytoken", req.Header.Get("Authorization"))
	}
}

func TestClientAuthAPIKey(t *testing.T) {
	c := New("localhost", 9600)
	c.SetAPIKey("mykey")

	req, _ := http.NewRequest("GET", "/test", nil)
	c.auth(req)

	if req.Header.Get("X-API-Key") != "mykey" {
		t.Errorf("X-API-Key = %q, want mykey", req.Header.Get("X-API-Key"))
	}
}

func TestClientAuthPrecedence(t *testing.T) {
	c := New("localhost", 9600)
	c.token = "mytoken"
	c.SetAPIKey("mykey")

	req, _ := http.NewRequest("GET", "/test", nil)
	c.auth(req)

	if req.Header.Get("Authorization") != "Bearer mytoken" {
		t.Error("token should take precedence over API key")
	}
}

func TestClientLogin(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"token":"test-token","token_type":"bearer"}`))
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
	}

	err := c.Login("admin", "admin")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}
	if c.token != "test-token" {
		t.Errorf("token = %q, want test-token", c.token)
	}
}

func TestClientLoginError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid credentials"}`))
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
	}

	err := c.Login("admin", "wrong")
	if err == nil {
		t.Fatal("expected error for failed login")
	}
}

func TestClientLoginEmptyToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"token":"","token_type":"bearer"}`))
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
	}

	err := c.Login("admin", "admin")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestClientLoginServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
	}

	err := c.Login("admin", "admin")
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestClientQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"columns":["id"],"rows":[[1]]}`))
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
		token:      "test-token",
	}

	resp, err := c.Query("main", "SELECT * FROM t")
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(resp.Columns) != 1 || resp.Columns[0] != "id" {
		t.Errorf("Columns = %v, want [id]", resp.Columns)
	}
}

func TestClientQueryRaw(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"columns":["id"],"rows":[[1]]}`))
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
		token:      "test-token",
	}

	raw, err := c.QueryRaw("main", "SELECT * FROM t")
	if err != nil {
		t.Fatalf("QueryRaw() error: %v", err)
	}
	if raw == "" {
		t.Fatal("raw response should not be empty")
	}
}

func TestClientTransaction(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"columns":["last_insert_id","rows_affected"],"rows":[[1,1]]}]}`))
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
		token:      "test-token",
	}

	resp, err := c.Transaction("main", []string{"INSERT INTO t VALUES (1)"})
	if err != nil {
		t.Fatalf("Transaction() error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Errorf("got %d results, want 1", len(resp.Results))
	}
}

func TestClientListDatabases(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"databases":["main","analytics"]}`))
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
		token:      "test-token",
	}

	dbs, err := c.ListDatabases()
	if err != nil {
		t.Fatalf("ListDatabases() error: %v", err)
	}
	if len(dbs) != 2 {
		t.Errorf("got %d databases, want 2", len(dbs))
	}
}

func TestClientListDatabasesError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":"access denied"}`))
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
		token:      "test-token",
	}

	_, err := c.ListDatabases()
	if err == nil {
		t.Fatal("expected error for access denied")
	}
}

func TestClientQueryServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := &Client{
		baseURL:    ts.URL,
		httpClient: ts.Client(),
		token:      "test-token",
	}

	_, err := c.Query("main", "SELECT 1")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
