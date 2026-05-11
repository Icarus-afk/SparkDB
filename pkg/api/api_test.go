package api

import (
	"encoding/json"
	"testing"
)

func TestQueryRequestSerialization(t *testing.T) {
	req := QueryRequest{Query: "SELECT 1", Database: "main", Params: []interface{}{"a", 1}}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded QueryRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Query != "SELECT 1" {
		t.Errorf("Query = %q", decoded.Query)
	}
}

func TestErrorResponse(t *testing.T) {
	err := ErrorResponse{Error: "test error", Code: 400}
	data, _ := json.Marshal(err)
	var decoded ErrorResponse
	json.Unmarshal(data, &decoded)
	if decoded.Error != "test error" {
		t.Errorf("Error = %q", decoded.Error)
	}
	if decoded.Code != 400 {
		t.Errorf("Code = %d", decoded.Code)
	}
}

func TestMessageResponse(t *testing.T) {
	msg := MessageResponse{Message: "hello"}
	data, _ := json.Marshal(msg)
	var decoded MessageResponse
	json.Unmarshal(data, &decoded)
	if decoded.Message != "hello" {
		t.Errorf("Message = %q", decoded.Message)
	}
}

func TestCreateUserResponse(t *testing.T) {
	resp := CreateUserResponse{ID: 1, Username: "test", Role: "admin"}
	data, _ := json.Marshal(resp)
	var decoded CreateUserResponse
	json.Unmarshal(data, &decoded)
	if decoded.ID != 1 || decoded.Username != "test" || decoded.Role != "admin" {
		t.Error("CreateUserResponse round-trip failed")
	}
}

func TestUsersResponse(t *testing.T) {
	resp := UsersResponse{Users: []UserView{{ID: 1, Username: "u"}}}
	data, _ := json.Marshal(resp)
	var decoded UsersResponse
	json.Unmarshal(data, &decoded)
	if len(decoded.Users) != 1 {
		t.Errorf("Users = %d", len(decoded.Users))
	}
}

func TestHealthResponse(t *testing.T) {
	h := HealthResponse{Status: "ok", Checks: map[string]string{"db": "ok"}}
	data, _ := json.Marshal(h)
	var decoded HealthResponse
	json.Unmarshal(data, &decoded)
	if decoded.Status != "ok" {
		t.Errorf("Status = %q", decoded.Status)
	}
}

func TestAPIKeyResponse(t *testing.T) {
	r := APIKeyResponse{APIKey: "key123", Name: "test"}
	data, _ := json.Marshal(r)
	var decoded APIKeyResponse
	json.Unmarshal(data, &decoded)
	if decoded.APIKey != "key123" || decoded.Name != "test" {
		t.Error("APIKeyResponse round-trip failed")
	}
}

func TestAPIKeyResponseWithoutName(t *testing.T) {
	r := APIKeyResponse{APIKey: "key123"}
	data, _ := json.Marshal(r)
	var decoded APIKeyResponse
	json.Unmarshal(data, &decoded)
	if decoded.APIKey != "key123" || decoded.Name != "" {
		t.Error("Name should be empty")
	}
}
