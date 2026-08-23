package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port           string
	Frontend       string
	MySQLDSN       string        // MySQL 连接字符串（必填）
	JWTSecret      string
	AllowedOrigins string        // CORS allowed origins, comma-separated, "*" for all
	ReadTimeout    time.Duration // HTTP read timeout
	WriteTimeout   time.Duration // HTTP write timeout
	IdleTimeout    time.Duration // HTTP idle timeout

	// TrustedProxies 可信反向代理 IP 列表。当服务部署在 Nginx/Traefik 等反代之后时，
	// 必须在此列出代理 IP，否则 c.ClientIP() 会拿到代理 IP 导致限流/锁定失效。
	TrustedProxies []string
}

const minJWTSecretLen = 32

func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	mysqlDSN := os.Getenv("MYSQL_DSN")
	if mysqlDSN == "" {
		return nil, fmt.Errorf("MYSQL_DSN 环境变量未设置（如 user:pass@tcp(host:3306)/dbname?parseTime=true&charset=utf8mb4&loc=Local）")
	}

	frontend := os.Getenv("FRONTEND_DIR")
	if frontend == "" {
		frontend = "./frontend/dist"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET 环境变量未设置，请设置至少 %d 位的随机字符串", minJWTSecretLen)
	}
	if len(jwtSecret) < minJWTSecretLen {
		return nil, fmt.Errorf("JWT_SECRET 长度不足 %d 位（当前 %d 位），请使用更长的密钥", minJWTSecretLen, len(jwtSecret))
	}

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "*"
	}

	cfg := &Config{
		Port:           port,
		Frontend:       frontend,
		MySQLDSN:       mysqlDSN,
		JWTSecret:      jwtSecret,
		AllowedOrigins: allowedOrigins,
		ReadTimeout:    getDuration("HTTP_READ_TIMEOUT", 10*time.Second),
		WriteTimeout:   getDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:    getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		TrustedProxies: parseCSV(os.Getenv("TRUSTED_PROXIES")),
	}
	return cfg, nil
}

// parseCSV 解析逗号分隔的字符串为去空格后的切片。
func parseCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
		slog.Warn("环境变量解析失败，使用默认值", "key", key, "value", v, "error", err)
	}
	return def
}
