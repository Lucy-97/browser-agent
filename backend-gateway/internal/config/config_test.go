package config

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnvOrFile(t *testing.T) {
	secretFile := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secretFile, []byte("  file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_SECRET", "")
	t.Setenv("TEST_SECRET_FILE", secretFile)
	if got := getEnvOrFile("TEST_SECRET", "fallback"); got != "file-secret" {
		t.Fatalf("getEnvOrFile() = %q, want file-secret", got)
	}

	t.Setenv("TEST_SECRET", "environment-secret")
	if got := getEnvOrFile("TEST_SECRET", "fallback"); got != "environment-secret" {
		t.Fatalf("environment value should take precedence, got %q", got)
	}
}

func TestRedisTLSConfig(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		got, err := (RedisConfig{}).TLSConfig()
		if err != nil || got != nil {
			t.Fatalf("TLSConfig() = %#v, %v; want nil, nil", got, err)
		}
	})

	t.Run("uses configured server name", func(t *testing.T) {
		got, err := (RedisConfig{
			Host:          "10.0.0.8",
			TLSEnabled:    true,
			TLSServerName: "cache.example.com",
		}).TLSConfig()
		if err != nil {
			t.Fatal(err)
		}
		if got.ServerName != "cache.example.com" || got.MinVersion != tls.VersionTLS12 {
			t.Fatalf("unexpected TLS config: %#v", got)
		}
	})

	t.Run("rejects unreadable CA", func(t *testing.T) {
		_, err := (RedisConfig{
			Host:       "cache.example.com",
			TLSEnabled: true,
			TLSCAFile:  filepath.Join(t.TempDir(), "missing.pem"),
		}).TLSConfig()
		if err == nil {
			t.Fatal("expected unreadable CA error")
		}
	})
}
