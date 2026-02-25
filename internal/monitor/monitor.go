package monitor

import (
	"os"
	"runtime"
	"sync"
	"time"
)

type DBStatusProvider interface {
	List() []string
}

type Monitor struct {
	startTime    time.Time
	mu           sync.RWMutex
	totalQueries int64
	failedLogins int64
	latencies    []time.Duration
	latCap       int
	dbProvider   DBStatusProvider
}

type Stats struct {
	UptimeSeconds   float64        `json:"uptime_seconds"`
	TotalQueries    int64          `json:"total_queries"`
	FailedLogins    int64          `json:"failed_logins"`
	ActiveConns     int            `json:"active_connections"`
	AvgLatencyMs    float64        `json:"avg_query_latency_ms"`
	P99LatencyMs    float64        `json:"p99_query_latency_ms"`
	Goroutines      int            `json:"goroutines"`
	AllocMB         float64        `json:"alloc_mb"`
	SysMB           float64        `json:"sys_mb"`
	NumDatabases    int            `json:"num_databases"`
	Databases       []DatabaseStat `json:"databases"`
}

type DatabaseStat struct {
	Name string `json:"name"`
	Size int64  `json:"size_bytes"`
}

func New(dbProvider DBStatusProvider) *Monitor {
	return &Monitor{
		startTime:  time.Now(),
		latencies:  make([]time.Duration, 0, 1000),
		latCap:     10000,
		dbProvider: dbProvider,
	}
}

func (m *Monitor) RecordQuery(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalQueries++
	m.latencies = append(m.latencies, duration)
	if len(m.latencies) > m.latCap {
		m.latencies = m.latencies[len(m.latencies)-m.latCap:]
	}
}

func (m *Monitor) RecordFailedLogin() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failedLogins++
}

func (m *Monitor) Stats() Stats {
	m.mu.RLock()
	totalQ := m.totalQueries
	failed := m.failedLogins
	lats := make([]time.Duration, len(m.latencies))
	copy(lats, m.latencies)
	m.mu.RUnlock()

	var avgLat, p99Lat float64
	if len(lats) > 0 {
		var sum time.Duration
		for _, l := range lats {
			sum += l
		}
		avgLat = float64(sum.Microseconds()) / float64(len(lats)) / 1000.0

		sorted := make([]time.Duration, len(lats))
		copy(sorted, lats)
		quickSort(sorted)
		p99Idx := int(float64(len(sorted)) * 0.99)
		if p99Idx >= len(sorted) {
			p99Idx = len(sorted) - 1
		}
		p99Lat = float64(sorted[p99Idx].Microseconds()) / 1000.0
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	dbNames := m.dbProvider.List()
	dbStats := make([]DatabaseStat, 0, len(dbNames))
	for _, name := range dbNames {
		path := "." + "/" + name
		info, err := os.Stat(path)
		if err == nil {
			dbStats = append(dbStats, DatabaseStat{Name: name, Size: info.Size()})
		}
	}

	return Stats{
		UptimeSeconds: time.Since(m.startTime).Seconds(),
		TotalQueries:  totalQ,
		FailedLogins:  failed,
		ActiveConns:   len(dbNames),
		AvgLatencyMs:  avgLat,
		P99LatencyMs:  p99Lat,
		Goroutines:    runtime.NumGoroutine(),
		AllocMB:       float64(memStats.Alloc) / 1024 / 1024,
		SysMB:         float64(memStats.Sys) / 1024 / 1024,
		NumDatabases:  len(dbNames),
		Databases:     dbStats,
	}
}

func quickSort(a []time.Duration) {
	if len(a) < 2 {
		return
	}
	left, right := 0, len(a)-1
	pivot := len(a) / 2
	a[pivot], a[right] = a[right], a[pivot]
	for i := range a {
		if a[i] < a[right] {
			a[i], a[left] = a[left], a[i]
			left++
		}
	}
	a[left], a[right] = a[right], a[left]
	quickSort(a[:left])
	quickSort(a[left+1:])
}
