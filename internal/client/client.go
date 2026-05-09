package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"sparkdb/pkg/api"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	apiKey     string
}

func New(host string, port int) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Login(username, password string) error {
	body, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	resp, err := c.httpClient.Post(c.baseURL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Token string `json:"token"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if result.Error != "" {
		return fmt.Errorf("%s", result.Error)
	}
	if result.Token == "" {
		return fmt.Errorf("login failed")
	}
	c.token = result.Token
	return nil
}

func (c *Client) SetAPIKey(key string) {
	c.apiKey = key
}

func (c *Client) Query(database, query string) (*api.QueryResponse, error) {
	req := api.QueryRequest{Query: query, Database: database}
	body, _ := json.Marshal(req)

	httpReq, _ := http.NewRequest("POST", c.baseURL+"/query", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	c.auth(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	var qr api.QueryResponse
	raw, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(raw, &qr); err != nil {
		return nil, fmt.Errorf("decode: %w (body: %s)", err, string(raw))
	}
	return &qr, nil
}

func (c *Client) QueryRaw(database, query string) (string, error) {
	req := api.QueryRequest{Query: query, Database: database}
	body, _ := json.Marshal(req)

	httpReq, _ := http.NewRequest("POST", c.baseURL+"/query", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	c.auth(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	return string(raw), nil
}

func (c *Client) Transaction(database string, queries []string) (*api.TransactionResponse, error) {
	req := api.TransactionRequest{Queries: queries, Database: database}
	body, _ := json.Marshal(req)

	httpReq, _ := http.NewRequest("POST", c.baseURL+"/transaction", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	c.auth(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	var tr api.TransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &tr, nil
}

func (c *Client) ListDatabases() ([]string, error) {
	httpReq, _ := http.NewRequest("GET", c.baseURL+"/databases", nil)
	c.auth(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Databases []string `json:"databases"`
		Error     string   `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("%s", result.Error)
	}
	return result.Databases, nil
}

func (c *Client) auth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
}
