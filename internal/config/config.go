package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Auth        AuthConfig        `mapstructure:"auth"`
	TLS         TLSConfig         `mapstructure:"tls"`
	Encryption  EncryptionConfig  `mapstructure:"encryption"`
	Backup      BackupConfig      `mapstructure:"backup"`
	Replication ReplicationConfig `mapstructure:"replication"`
}

type ServerConfig struct {
	Host           string   `mapstructure:"host"`
	Port           int      `mapstructure:"port"`
	ReadOnly       bool     `mapstructure:"read_only"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

type DatabaseConfig struct {
	DataDir  string `mapstructure:"data_dir"`
	WALMode  bool   `mapstructure:"wal_mode"`
	MaxConns int    `mapstructure:"max_connections"`
}

type AuthConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	JWTSecret string `mapstructure:"jwt_secret"`
}

type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	AutoCert bool   `mapstructure:"auto_cert"`
}

type EncryptionConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Key      string `mapstructure:"key"`
	KeyFile  string `mapstructure:"key_file"`
}

type BackupConfig struct {
	Dir       string `mapstructure:"dir"`
	Schedule  string `mapstructure:"schedule"`
	KeepCount int    `mapstructure:"keep_count"`
}

type ReplicationConfig struct {
	Role         string `mapstructure:"role"`
	PrimaryURL   string `mapstructure:"primary_url"`
	APIKey       string `mapstructure:"api_key"`
	PollInterval int    `mapstructure:"poll_interval"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/sparkdb")
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".config", "sparkdb"))
		}
	}

	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 9600)
	v.SetDefault("database.data_dir", ".")
	v.SetDefault("database.wal_mode", true)
	v.SetDefault("database.max_connections", 100)
	v.SetDefault("tls.enabled", false)
	v.SetDefault("tls.auto_cert", true)
	v.SetDefault("tls.cert_file", "sparkdb.crt")
	v.SetDefault("tls.key_file", "sparkdb.key")
	v.SetDefault("encryption.enabled", false)
	v.SetDefault("backup.dir", "backups")
	v.SetDefault("backup.schedule", "")
	v.SetDefault("backup.keep_count", 10)

	v.SetDefault("replication.role", "standalone")
	v.SetDefault("replication.poll_interval", 5)

	v.SetEnvPrefix("SPARKDB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return nil, fmt.Errorf("invalid server port: %d (must be 1-65535)", cfg.Server.Port)
	}
	if cfg.Database.MaxConns < 1 {
		cfg.Database.MaxConns = 1
	}
	if cfg.Backup.KeepCount < 0 {
		cfg.Backup.KeepCount = 0
	}
	if cfg.TLS.Enabled && !cfg.TLS.AutoCert {
		if cfg.TLS.CertFile == "" || cfg.TLS.KeyFile == "" {
			return nil, fmt.Errorf("tls enabled without auto_cert: cert_file and key_file must be set")
		}
	}
	if cfg.Encryption.Enabled {
		if cfg.Encryption.Key == "" && cfg.Encryption.KeyFile == "" {
			return nil, fmt.Errorf("encryption enabled but no key or key_file provided")
		}
	}

	switch cfg.Replication.Role {
	case "primary", "replica", "standalone":
	case "":
		cfg.Replication.Role = "standalone"
	default:
		return nil, fmt.Errorf("invalid replication role %q: must be 'primary', 'replica', or 'standalone'", cfg.Replication.Role)
	}
	if cfg.Replication.Role == "replica" && cfg.Replication.PrimaryURL == "" {
		return nil, fmt.Errorf("replica role requires primary_url")
	}
	if cfg.Replication.Role == "replica" && cfg.Replication.APIKey == "" {
		return nil, fmt.Errorf("replica role requires api_key for authentication with primary")
	}
	if cfg.Replication.PollInterval < 1 {
		cfg.Replication.PollInterval = 5
	}

	return &cfg, nil
}
