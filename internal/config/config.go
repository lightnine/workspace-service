package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig
	Workspace WorkspaceConfig
	Log       LogConfig
	MySQL     MySQLConfig
	Gateway   GatewayConfig
}

type ServerConfig struct {
	Name            string
	Address         string
	Mode            string
	URLPrefix       string
	ShutdownTimeout time.Duration
}

type WorkspaceConfig struct {
	MountRoot string
}

type LogConfig struct {
	Level       string
	Encoding    string
	Development bool
	OutputPaths []string
}

type MySQLConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// GatewayConfig configures the transparent reverse proxy that forwards Jupyter
// REST (/api/sessions, /api/kernels, /api/kernelspecs) and kernel channel
// WebSocket traffic to a remote Jupyter Gateway / Enterprise Gateway server.
//
// The proxy is disabled when URL is empty.
type GatewayConfig struct {
	// URL is the gateway HTTP base URL, e.g. http://dlc-enterprise-gateway:8888.
	URL string
	// WSURL is the gateway WebSocket base URL. When empty it is derived from URL
	// (http->ws, https->wss).
	WSURL string

	// AuthHeaderKey is the header used to carry the auth token (default Authorization).
	AuthHeaderKey string
	// AuthScheme prefixes the token value, e.g. "token" -> "Authorization: token <t>".
	// When empty the raw token is sent without a scheme prefix.
	AuthScheme string
	// AuthToken is the gateway auth token. When empty no auth header is injected.
	AuthToken string
	// Headers are extra static headers added to every forwarded request.
	Headers map[string]string

	// ConnectTimeout bounds the TCP/TLS connection setup to the gateway.
	ConnectTimeout time.Duration
	// RequestTimeout bounds a full REST request/response round trip.
	RequestTimeout time.Duration
	// MaxRequestRetries is the max retry count for idempotent (GET/DELETE) requests.
	MaxRequestRetries int

	// ValidateCert toggles TLS certificate verification against the gateway.
	ValidateCert bool
	// CACerts is an optional path to a PEM bundle used to verify the gateway.
	CACerts string
	// ClientCert / ClientKey enable mutual TLS when both are set.
	ClientCert string
	ClientKey  string
}

func Load(configFile string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetConfigType("yaml")
	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName("workspace-service")
		v.AddConfigPath("./conf")
		v.AddConfigPath(".")
	}

	v.SetEnvPrefix("WORKSPACE_SERVICE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Name:            v.GetString("server.name"),
			Address:         v.GetString("server.address"),
			Mode:            v.GetString("server.mode"),
			URLPrefix:       v.GetString("server.url_prefix"),
			ShutdownTimeout: v.GetDuration("server.shutdown_timeout"),
		},
		Workspace: WorkspaceConfig{
			MountRoot: v.GetString("workspace.mount_root"),
		},
		Log: LogConfig{
			Level:       v.GetString("log.level"),
			Encoding:    v.GetString("log.encoding"),
			Development: v.GetBool("log.development"),
			OutputPaths: v.GetStringSlice("log.output_paths"),
		},
		MySQL: MySQLConfig{
			DSN:             v.GetString("mysql.dsn"),
			MaxOpenConns:    v.GetInt("mysql.max_open_conns"),
			MaxIdleConns:    v.GetInt("mysql.max_idle_conns"),
			ConnMaxLifetime: v.GetDuration("mysql.conn_max_lifetime"),
		},
		Gateway: GatewayConfig{
			URL:               v.GetString("gateway.url"),
			WSURL:             v.GetString("gateway.ws_url"),
			AuthHeaderKey:     v.GetString("gateway.auth_header_key"),
			AuthScheme:        v.GetString("gateway.auth_scheme"),
			AuthToken:         v.GetString("gateway.auth_token"),
			Headers:           v.GetStringMapString("gateway.headers"),
			ConnectTimeout:    v.GetDuration("gateway.connect_timeout"),
			RequestTimeout:    v.GetDuration("gateway.request_timeout"),
			MaxRequestRetries: v.GetInt("gateway.max_request_retries"),
			ValidateCert:      v.GetBool("gateway.validate_cert"),
			CACerts:           v.GetString("gateway.ca_certs"),
			ClientCert:        v.GetString("gateway.client_cert"),
			ClientKey:         v.GetString("gateway.client_key"),
		},
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.name", "workspace-service")
	v.SetDefault("server.address", ":8080")
	v.SetDefault("server.mode", "release")
	v.SetDefault("server.url_prefix", "")
	v.SetDefault("server.shutdown_timeout", "10s")

	v.SetDefault("workspace.mount_root", "~/mnt/studio")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.encoding", "json")
	v.SetDefault("log.development", false)
	v.SetDefault("log.output_paths", []string{"stdout"})

	v.SetDefault("mysql.max_open_conns", 20)
	v.SetDefault("mysql.max_idle_conns", 10)
	v.SetDefault("mysql.conn_max_lifetime", "30m")

	v.SetDefault("gateway.url", "")
	v.SetDefault("gateway.ws_url", "")
	v.SetDefault("gateway.auth_header_key", "Authorization")
	v.SetDefault("gateway.auth_scheme", "token")
	v.SetDefault("gateway.auth_token", "")
	v.SetDefault("gateway.connect_timeout", "40s")
	v.SetDefault("gateway.request_timeout", "42s")
	v.SetDefault("gateway.max_request_retries", 2)
	v.SetDefault("gateway.validate_cert", true)
}
