package api

import "time"

type QueryRequest struct {
	Query    string        `json:"query"`
	Database string        `json:"database"`
	Params   []interface{} `json:"params,omitempty"`
}

type QueryResponse struct {
	Columns []string        `json:"columns,omitempty"`
	Rows    [][]interface{} `json:"rows,omitempty"`
	Error   string          `json:"error,omitempty"`
	Time    string          `json:"time,omitempty"`
}

type TransactionRequest struct {
	Queries  []string `json:"queries"`
	Database string   `json:"database"`
}

type TransactionResponse struct {
	Results []QueryResponse `json:"results"`
	Error   string          `json:"error,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type CreateUserResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type UserView struct {
	ID          int64      `json:"id"`
	Username    string     `json:"username"`
	Role        string     `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	LockedUntil *time.Time `json:"locked_until,omitempty"`
}

type UsersResponse struct {
	Users []UserView `json:"users"`
}

type APIKeysResponse struct {
	APIKeys []interface{} `json:"api_keys"`
}

type AuditLogsResponse struct {
	Logs []interface{} `json:"logs"`
}

type DatabasesResponse struct {
	Databases []string `json:"databases"`
}

type APIKeyResponse struct {
	APIKey string `json:"api_key"`
	Name   string `json:"name,omitempty"`
}

type BackupsResponse struct {
	Backups []interface{} `json:"backups"`
}

type HealthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

type APIKeyListResponse struct {
	APIKeys []interface{} `json:"api_keys"`
}

type ReplicationLogResponse struct {
	Entries []interface{} `json:"entries"`
}

type RestoreResponse struct {
	Message  string `json:"message"`
	Database string `json:"database"`
}

type CreateAPIKeyRequest struct {
	Name string `json:"name"`
}

type UpdateRoleRequest struct {
	Role string `json:"role"`
}

type PasswordRequest struct {
	Password string `json:"password"`
}

type DatabaseRequest struct {
	Database string `json:"database"`
}

type BackupFileRequest struct {
	BackupFile string `json:"backup_file"`
	Database   string `json:"database"`
}
