package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Auth       AuthConfig       `mapstructure:"auth"`
	TLS        TLSConfig        `mapstructure:"tls"`
	Encryption EncryptionConfig `mapstructure:"encryption"`
	Backup     BackupConfig     `mapstructure:"backup"`
}

type ServerConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	ReadOnly bool   `mapstructure:"read_only"`
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

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("json")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/vaultlite")

	if path != "" {
		v.SetConfigFile(path)
	}

	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 9600)
	v.SetDefault("database.data_dir", ".")
	v.SetDefault("database.wal_mode", true)
	v.SetDefault("database.max_connections", 100)
	v.SetDefault("tls.enabled", false)
	v.SetDefault("tls.auto_cert", true)
	v.SetDefault("tls.cert_file", "vaultlite.crt")
	v.SetDefault("tls.key_file", "vaultlite.key")
	v.SetDefault("encryption.enabled", false)
	v.SetDefault("backup.dir", "backups")
	v.SetDefault("backup.schedule", "")
	v.SetDefault("backup.keep_count", 10)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
