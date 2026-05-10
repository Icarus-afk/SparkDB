package api

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
