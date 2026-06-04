package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadReadsYAMLAndAppliesDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "workspace-service.yaml")
	content := []byte(`
server:
  name: test-service
  address: 127.0.0.1:19090
  url_prefix: /api
workspace:
  mount_root: /tmp/test-studio
log:
  level: debug
  encoding: console
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Name != "test-service" {
		t.Fatalf("server name = %q, want %q", cfg.Server.Name, "test-service")
	}
	if cfg.Server.Address != "127.0.0.1:19090" {
		t.Fatalf("server address = %q, want %q", cfg.Server.Address, "127.0.0.1:19090")
	}
	if cfg.Server.URLPrefix != "/api" {
		t.Fatalf("server url prefix = %q, want %q", cfg.Server.URLPrefix, "/api")
	}
	if cfg.Server.ShutdownTimeout != 10*time.Second {
		t.Fatalf("shutdown timeout = %s, want 10s", cfg.Server.ShutdownTimeout)
	}
	if cfg.Workspace.MountRoot != "/tmp/test-studio" {
		t.Fatalf("workspace mount root = %q, want %q", cfg.Workspace.MountRoot, "/tmp/test-studio")
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("log level = %q, want %q", cfg.Log.Level, "debug")
	}
	if cfg.Log.Encoding != "console" {
		t.Fatalf("log encoding = %q, want %q", cfg.Log.Encoding, "console")
	}
}

func TestLoadDefaultsWorkspaceMountRoot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "workspace-service.yaml")
	content := []byte(`
server:
  name: test-service
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := filepath.Join("~", "mnt", "studio")
	if cfg.Workspace.MountRoot != want {
		t.Fatalf("workspace mount root = %q, want %q", cfg.Workspace.MountRoot, want)
	}
}
