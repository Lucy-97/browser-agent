package config

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestEnvOrFile(t *testing.T) {
	secretFile := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secretFile, []byte("  file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_SECRET", "")
	t.Setenv("TEST_SECRET_FILE", secretFile)
	if got := envOrFile("TEST_SECRET"); got != "file-secret" {
		t.Fatalf("envOrFile() = %q, want file-secret", got)
	}

	t.Setenv("TEST_SECRET", "environment-secret")
	if got := envOrFile("TEST_SECRET"); got != "environment-secret" {
		t.Fatalf("environment value should take precedence, got %q", got)
	}
}

func TestRedisTLSConfig(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		got, err := (Config{}).RedisTLSConfig()
		if err != nil || got != nil {
			t.Fatalf("RedisTLSConfig() = %#v, %v; want nil, nil", got, err)
		}
	})

	t.Run("derives server name", func(t *testing.T) {
		got, err := (Config{RedisTLSEnabled: true, RedisAddr: "cache.example.com:6380"}).RedisTLSConfig()
		if err != nil {
			t.Fatal(err)
		}
		if got.ServerName != "cache.example.com" || got.MinVersion != tls.VersionTLS12 {
			t.Fatalf("unexpected TLS config: %#v", got)
		}
	})

	t.Run("rejects unreadable CA", func(t *testing.T) {
		_, err := (Config{
			RedisTLSEnabled: true,
			RedisAddr:       "cache.example.com:6380",
			RedisTLSCAFile:  filepath.Join(t.TempDir(), "missing.pem"),
		}).RedisTLSConfig()
		if err == nil {
			t.Fatal("expected unreadable CA error")
		}
	})
}
