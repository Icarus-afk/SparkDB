package server

import (
	"context"
	"fmt"
	"log/slog"
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
	"sparkdb/internal/rbac"
	"sparkdb/internal/replication"
	"sparkdb/internal/web"
	"sparkdb/pkg/api"
)

type Server struct {
	httpServer    *http.Server
	dbManager     *database.Manager
	systemDB      *database.SystemDB
	authenticator *auth.Authenticator
	backupMgr     *backup.Manager
	replEngine    *replication.Engine
	cfg           *config.Config
	tlsEnabled    bool
	stopSched     chan struct{}
}

func New(cfg *config.Config) (*Server, error) {
	var ciph *encryption.Cipher
	if cfg.Encryption.Enabled {
		var err error
		ciph, err = encryption.GetCipher(cfg.Encryption.Key, cfg.Encryption.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("encryption: %w", err)
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

	if err := systemDB.InitReplicationState(cfg.Replication.Role, cfg.Replication.PrimaryURL); err != nil {
		return nil, fmt.Errorf("init replication state: %w", err)
	}
	replEngine := replication.NewEngine(systemDB, executor, cfg.Replication.Role, cfg.Replication.PrimaryURL, cfg.Replication.APIKey, cfg.Replication.PollInterval)

	handler := NewHandler(executor, authenticator, systemDB, backupMgr, mon)
	handler.replEngine = replEngine

	rateLimiter := query.NewRateLimiter(60, time.Minute)
	requireAuth := authMiddleware(authenticator)
	optionalAuth := optionalAuthMiddleware(authenticator)

	mux := http.NewServeMux()

	mux.Handle("POST /query", requireAuth(http.HandlerFunc(handler.HandleQuery)))
	mux.Handle("POST /transaction", requireAuth(http.HandlerFunc(handler.HandleTransaction)))
	mux.Handle("POST /backup", requireAuth(http.HandlerFunc(handler.HandleBackup)))
	mux.Handle("POST /restore", requireAuth(http.HandlerFunc(handler.HandleRestore)))
	mux.Handle("GET /backups", requireAuth(http.HandlerFunc(handler.HandleListBackups)))
	mux.Handle("DELETE /backups/{name}", requireAuth(http.HandlerFunc(handler.HandleDeleteBackup)))

	mux.Handle("POST /auth/login", optionalAuth(http.HandlerFunc(handler.HandleLogin)))
	mux.Handle("POST /auth/api-keys", requireAuth(http.HandlerFunc(handler.HandleCreateAPIKey)))
	mux.Handle("GET /auth/api-keys", requireAuth(http.HandlerFunc(handler.HandleListAPIKeys)))
	mux.Handle("DELETE /auth/api-keys/{id}", requireAuth(http.HandlerFunc(handler.HandleDeleteAPIKey)))
	mux.Handle("POST /auth/api-keys/{id}/reveal", requireAuth(http.HandlerFunc(handler.HandleRevealAPIKey)))

	mux.Handle("POST /admin/users", requireAuth(http.HandlerFunc(handler.HandleCreateUser)))
	mux.Handle("GET /admin/users", requireAuth(http.HandlerFunc(handler.HandleListUsers)))
	mux.Handle("PUT /admin/users/{id}/role", requireAuth(http.HandlerFunc(handler.HandleUpdateUserRole)))
	mux.Handle("PUT /admin/users/{id}/password", requireAuth(http.HandlerFunc(handler.HandleUpdateUserPassword)))
	mux.Handle("DELETE /admin/users/{id}", requireAuth(http.HandlerFunc(handler.HandleDeleteUser)))
	mux.Handle("GET /admin/audit-logs", requireAuth(http.HandlerFunc(handler.HandleAuditLogs)))

	mux.Handle("GET /health", optionalAuth(http.HandlerFunc(handler.HandleHealth)))
	mux.Handle("GET /databases", requireAuth(http.HandlerFunc(handler.HandleListDatabases)))
	mux.Handle("GET /stats", requireAuth(http.HandlerFunc(handler.HandleStats)))
	mux.Handle("GET /metrics", optionalAuth(http.HandlerFunc(handler.HandlePrometheus)))
	mux.Handle("GET /replication/log", requireAuth(http.HandlerFunc(handler.HandleReplicationLog)))

	mux.Handle("GET /", web.NewHandler())

	var h http.Handler = mux
	h = loggingMiddleware(h)
	h = recoveryMiddleware(h)
	h = bodyLimitMiddleware(1 << 20)(h)
	h = corsMiddleware(cfg.Server.AllowedOrigins)(h)
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

	mux.Handle("POST /shutdown", requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.UserFromContext(r.Context())
		if user == nil || !rbac.HasPermission(rbac.Role(user.Role), rbac.PermCreateUser) {
			writeJSON(w, http.StatusForbidden, api.ErrorResponse{Error: "only admins can shutdown", Code: 403})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "shutting down"})
		slog.Warn("shutdown requested via API")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			httpServer.Shutdown(ctx)
			dbManager.CloseAll()
			systemDB.Close()
		}()
	})))

	return &Server{
		httpServer:    httpServer,
		dbManager:     dbManager,
		systemDB:      systemDB,
		authenticator: authenticator,
		backupMgr:     backupMgr,
		replEngine:    replEngine,
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
		slog.Info("server starting", "addr", "https://"+s.httpServer.Addr)
	} else {
		slog.Info("server starting", "addr", "http://"+s.httpServer.Addr)
	}

	if s.cfg.Backup.Schedule != "" {
		go s.scheduledBackupLoop()
	}

	s.replEngine.Start()

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
	slog.Info("server shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	close(s.stopSched)
	s.replEngine.Stop()
	s.httpServer.Shutdown(ctx)
	s.dbManager.CloseAll()
	s.systemDB.Close()
	slog.Info("server stopped")
}

func (s *Server) Authenticator() *auth.Authenticator {
	return s.authenticator
}

func (s *Server) scheduledBackupLoop() {
	d, err := time.ParseDuration(s.cfg.Backup.Schedule)
	if err != nil {
		slog.Error("invalid backup schedule", "schedule", s.cfg.Backup.Schedule, "error", err)
		return
	}
	if d < time.Minute {
		d = time.Minute
	}

	ticker := time.NewTicker(d)
	defer ticker.Stop()

	slog.Info("scheduled backups enabled", "interval", d)

	dbNames := []string{"main"}

	for {
		select {
		case <-ticker.C:
			for _, name := range dbNames {
				info, err := s.backupMgr.CreateBackup(name)
				if err != nil {
					slog.Error("scheduled backup failed", "database", name, "error", err)
					continue
				}
				slog.Info("scheduled backup created", "name", info.Name, "size", info.Size)
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
			slog.Error("failed to prune backup", "name", b.Name, "error", err)
		} else {
			slog.Info("pruned old backup", "name", b.Name)
		}
	}
}
