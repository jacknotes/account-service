package config

import (
	"strings"
	"testing"
)

func TestLoad_RequiresMySQLDSN(t *testing.T) {
	t.Setenv("MYSQL_DSN", "")
	t.Setenv("JWT_SECRET", "x")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "MYSQL_DSN") {
		t.Errorf("Load() without MYSQL_DSN should error, got %v", err)
	}
}

func TestLoad_RequiresJWTSecret(t *testing.T) {
	t.Setenv("MYSQL_DSN", "u:p@tcp(127.0.0.1:3306)/db")
	t.Setenv("JWT_SECRET", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Errorf("Load() without JWT_SECRET should error, got %v", err)
	}
}

func TestLoad_ShortJWTSecret(t *testing.T) {
	t.Setenv("MYSQL_DSN", "u:p@tcp(127.0.0.1:3306)/db")
	t.Setenv("JWT_SECRET", "short")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "长度不足") {
		t.Errorf("Load() with short JWT_SECRET should error, got %v", err)
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("MYSQL_DSN", "u:p@tcp(127.0.0.1:3306)/db")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("PORT", "")
	t.Setenv("ALLOWED_ORIGINS", "")
	t.Setenv("TRUSTED_PROXIES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.Port != "8081" {
		t.Errorf("Port = %q, want 8081", cfg.Port)
	}
	if cfg.AllowedOrigins != "*" {
		t.Errorf("AllowedOrigins = %q, want *", cfg.AllowedOrigins)
	}
	if cfg.Frontend != "./frontend/dist" {
		t.Errorf("Frontend = %q, want ./frontend/dist", cfg.Frontend)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("TrustedProxies = %v, want empty", cfg.TrustedProxies)
	}
}

func TestLoad_ParsesTrustedProxies(t *testing.T) {
	t.Setenv("MYSQL_DSN", "u:p@tcp(127.0.0.1:3306)/db")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 10.0.0.2")
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want 9090", cfg.Port)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0] != "10.0.0.1" || cfg.TrustedProxies[1] != "10.0.0.2" {
		t.Errorf("TrustedProxies = %v, want [10.0.0.1 10.0.0.2]", cfg.TrustedProxies)
	}
}
