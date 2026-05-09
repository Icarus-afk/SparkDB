package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sparkdb/internal/auth"
	"sparkdb/internal/backup"
	"sparkdb/internal/config"
	"sparkdb/internal/database"
	"sparkdb/internal/encryption"
	"sparkdb/internal/monitor"
	"sparkdb/internal/query"
)

type Server struct {
	httpServer    *http.Server
	dbManager     *database.Manager
	systemDB      *database.SystemDB
	authenticator *auth.Authenticator
	backupMgr     *backup.Manager
	cfg           *config.Config
	tlsEnabled    bool
	stopSched     chan struct{}
}

func New(cfg *config.Config) (*Server, error) {
	var ciph *encryption.Cipher
	if cfg.Encryption.Enabled {
		key := cfg.Encryption.Key
		if key == "" && cfg.Encryption.KeyFile != "" {
			keyData, err := os.ReadFile(cfg.Encryption.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("read encryption key file: %w", err)
			}
			key = string(keyData)
		}
		if key == "" {
			key = os.Getenv("SPARKDB_ENCRYPTION_KEY")
		}
		if key == "" {
			return nil, fmt.Errorf("encryption enabled but no key provided (set key, key_file, or SPARKDB_ENCRYPTION_KEY)")
		}

		var err error
		ciph, err = encryption.NewCipherFromHex(key)
		if err != nil {
			return nil, fmt.Errorf("init cipher: %w", err)
		}
	}

	var dbManager *database.Manager
	if ciph != nil {
		dbManager = database.NewEncryptedManager(cfg.Database.DataDir, cfg.Database.WALMode, cfg.Database.MaxConns, ciph)
	} else {
		dbManager = database.NewManager(cfg.Database.DataDir, cfg.Database.WALMode, cfg.Database.MaxConns)
	}

	systemDBPath := cfg.Database.DataDir + "/sparkdb_system.db"
	systemDB, err := database.NewSystemDB(systemDBPath)
	if err != nil {
		return nil, fmt.Errorf("init system database: %w", err)
	}

	authenticator := auth.NewAuthenticator(auth.AuthenticatorConfig{
		SystemDB:    systemDB,
		JWTSecret:   cfg.Auth.JWTSecret,
		SessionTTL:  24 * time.Hour,
		LoginLimit:  5,
		LockoutTime: 15 * time.Minute,
	})

	if err := authenticator.EnsureDefaultAdmin(); err != nil {
		return nil, fmt.Errorf("ensure default admin: %w", err)
	}

	mon := monitor.New(dbManager)
	executor := database.NewExecutorWithMonitor(dbManager, mon)

	backupMgr := backup.NewManager(cfg.Backup.Dir, cfg.Database.DataDir, dbManager, ciph)

	handler := NewHandler(executor, authenticator, systemDB, backupMgr, mon)

	rateLimiter := query.NewRateLimiter(60, time.Minute)
	requireAuth := authMiddleware(authenticator)
	optionalAuth := optionalAuthMiddleware(authenticator)

	mux := http.NewServeMux()

	mux.Handle("POST /query", requireAuth(http.HandlerFunc(handler.HandleQuery)))
	mux.Handle("POST /transaction", requireAuth(http.HandlerFunc(handler.HandleTransaction)))
	mux.Handle("POST /backup", requireAuth(http.HandlerFunc(handler.HandleBackup)))
	mux.Handle("POST /restore", requireAuth(http.HandlerFunc(handler.HandleRestore)))
	mux.Handle("GET /backups", requireAuth(http.HandlerFunc(handler.HandleListBackups)))

	mux.Handle("POST /auth/login", optionalAuth(http.HandlerFunc(handler.HandleLogin)))
	mux.Handle("POST /auth/api-keys", requireAuth(http.HandlerFunc(handler.HandleCreateAPIKey)))

	mux.Handle("POST /admin/users", requireAuth(http.HandlerFunc(handler.HandleCreateUser)))
	mux.Handle("GET /admin/users", requireAuth(http.HandlerFunc(handler.HandleListUsers)))
	mux.Handle("GET /admin/audit-logs", requireAuth(http.HandlerFunc(handler.HandleAuditLogs)))

	mux.Handle("GET /health", optionalAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})))
	mux.Handle("GET /stats", requireAuth(http.HandlerFunc(handler.HandleStats)))
	mux.Handle("GET /metrics", http.HandlerFunc(handler.HandlePrometheus))

	var h http.Handler = mux
	h = loggingMiddleware(h)
	h = recoveryMiddleware(h)
	h = corsMiddleware(h)
	h = rateLimitMiddleware(rateLimiter)(h)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	tlsEnabled := false
	if cfg.TLS.Enabled {
		tlsEnabled = true

		if cfg.TLS.AutoCert {
			if err := encryption.EnsureCertFiles(cfg.TLS.CertFile, cfg.TLS.KeyFile); err != nil {
				return nil, fmt.Errorf("ensure TLS certs: %w", err)
			}
		}

		tlsCfg, err := encryption.TLSServerConfig(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("TLS config: %w", err)
		}
		httpServer.TLSConfig = tlsCfg
	}

	return &Server{
		httpServer:    httpServer,
		dbManager:     dbManager,
		systemDB:      systemDB,
		authenticator: authenticator,
		backupMgr:     backupMgr,
		cfg:           cfg,
		tlsEnabled:    tlsEnabled,
		stopSched:     make(chan struct{}),
	}, nil
}

func (s *Server) Start() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)

	if s.tlsEnabled {
		log.Printf("SparkDB starting on https://%s", s.httpServer.Addr)
	} else {
		log.Printf("SparkDB starting on http://%s", s.httpServer.Addr)
	}
	log.Println("default admin credentials: admin / admin")

	if s.cfg.Backup.Schedule != "" {
		go s.scheduledBackupLoop()
	}

	go func() {
		if s.tlsEnabled {
			errCh <- s.httpServer.ListenAndServeTLS("", "")
		} else {
			errCh <- s.httpServer.ListenAndServe()
		}
	}()

	select {
	case <-quit:
		s.Shutdown()
		return nil
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}
}

func (s *Server) Shutdown() {
	log.Println("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	close(s.stopSched)
	s.httpServer.Shutdown(ctx)
	s.dbManager.CloseAll()
	s.systemDB.Close()
	log.Println("server stopped")
}

func (s *Server) Authenticator() *auth.Authenticator {
	return s.authenticator
}

func (s *Server) scheduledBackupLoop() {
	d, err := time.ParseDuration(s.cfg.Backup.Schedule)
	if err != nil {
		log.Printf("invalid backup schedule %q: %v", s.cfg.Backup.Schedule, err)
		return
	}
	if d < time.Minute {
		d = time.Minute
	}

	ticker := time.NewTicker(d)
	defer ticker.Stop()

	log.Printf("scheduled backups every %s", d)

	dbNames := []string{"main"}

	for {
		select {
		case <-ticker.C:
			for _, name := range dbNames {
				info, err := s.backupMgr.CreateBackup(name)
				if err != nil {
					log.Printf("scheduled backup failed for %s: %v", name, err)
					continue
				}
				log.Printf("scheduled backup created: %s (%d bytes)", info.Name, info.Size)
			}

			if s.cfg.Backup.KeepCount > 0 {
				s.pruneOldBackups()
			}
		case <-s.stopSched:
			return
		}
	}
}

func (s *Server) pruneOldBackups() {
	all, err := s.backupMgr.ListBackups()
	if err != nil {
		return
	}

	if len(all) <= s.cfg.Backup.KeepCount {
		return
	}

	toDelete := all[s.cfg.Backup.KeepCount:]
	for _, b := range toDelete {
		if err := s.backupMgr.DeleteBackup(b.Path); err != nil {
			log.Printf("failed to prune backup %s: %v", b.Name, err)
		} else {
			log.Printf("pruned old backup: %s", b.Name)
		}
	}
}
