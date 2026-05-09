package replication

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"sparkdb/internal/database"
)

func IsWriteQuery(query string) bool {
	upper := strings.ToUpper(strings.TrimSpace(query))
	return !strings.HasPrefix(upper, "SELECT") &&
		!strings.HasPrefix(upper, "PRAGMA") &&
		!strings.HasPrefix(upper, "EXPLAIN")
}

type Engine struct {
	systemDB   *database.SystemDB
	executor   *database.Executor
	role       string
	primaryURL string
	apiKey     string
	pollIntvl  time.Duration
	stopCh     chan struct{}
	client     *http.Client
}

func NewEngine(systemDB *database.SystemDB, executor *database.Executor, role, primaryURL, apiKey string, pollInterval int) *Engine {
	return &Engine{
		systemDB:   systemDB,
		executor:   executor,
		role:       role,
		primaryURL: strings.TrimRight(primaryURL, "/"),
		apiKey:     apiKey,
		pollIntvl:  time.Duration(pollInterval) * time.Second,
		stopCh:     make(chan struct{}),
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *Engine) Start() {
	if e.role == "primary" {
		log.Println("[replication] role: primary")
	} else if e.role == "replica" {
		log.Printf("[replication] role: replica, primary: %s", e.primaryURL)
		go e.replicaLoop()
	} else {
		log.Println("[replication] role: standalone")
	}
}

func (e *Engine) Stop() {
	close(e.stopCh)
}

func (e *Engine) Role() string {
	return e.role
}

func (e *Engine) PrimaryURL() string {
	return e.primaryURL
}

func (e *Engine) replicaLoop() {
	state, err := e.systemDB.GetReplicationState()
	if err != nil {
		log.Printf("[replication] failed to load state: %v", err)
		return
	}
	lastID := state.LastAppliedID
	log.Printf("[replication] starting from log id %d", lastID)

	ticker := time.NewTicker(e.pollIntvl)
	defer ticker.Stop()

	e.pollOnce(&lastID)

	for {
		select {
		case <-ticker.C:
			e.pollOnce(&lastID)
		case <-e.stopCh:
			log.Println("[replication] stopping replica loop")
			return
		}
	}
}

type replicationLogResponse struct {
	Entries []*database.ReplicationEntry `json:"entries"`
}

func (e *Engine) pollOnce(lastID *int64) {
	url := fmt.Sprintf("%s/replication/log?since=%d&limit=500", e.primaryURL, *lastID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("[replication] create request: %v", err)
		return
	}
	if e.apiKey != "" {
		req.Header.Set("X-API-Key", e.apiKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		log.Printf("[replication] poll failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[replication] primary returned %d: %s", resp.StatusCode, string(body))
		return
	}

	var result replicationLogResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[replication] decode response: %v", err)
		return
	}

	if len(result.Entries) == 0 {
		return
	}

	for _, entry := range result.Entries {
		if err := e.applyEntry(entry); err != nil {
			log.Printf("[replication] apply entry %d failed: %v", entry.ID, err)
			return
		}
		*lastID = entry.ID
	}

	if err := e.systemDB.UpdateReplicationAppliedID(*lastID); err != nil {
		log.Printf("[replication] update state: %v", err)
	}

	if len(result.Entries) >= 500 {
		e.pollOnce(lastID)
	}
}

func (e *Engine) applyEntry(entry *database.ReplicationEntry) error {
	_, err := e.executor.Execute(entry.DatabaseName, entry.Query)
	if err != nil {
		return fmt.Errorf("execute on %s: %w", entry.DatabaseName, err)
	}
	return nil
}
