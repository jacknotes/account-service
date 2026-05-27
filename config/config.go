package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Port           string
	Database       string
	Frontend       string
	JWTSecret      string
	AllowedOrigins string        // CORS allowed origins, comma-separated, "*" for all
	ReadTimeout    time.Duration // HTTP read timeout
	WriteTimeout   time.Duration // HTTP write timeout
	IdleTimeout    time.Duration // HTTP idle timeout
}

const minJWTSecretLen = 32

func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/accounting.db"
	}
	frontend := os.Getenv("FRONTEND_DIR")
	if frontend == "" {
		frontend = "./frontend"
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
	return &Config{
		Port:           port,
		Database:       dbPath,
		Frontend:       frontend,
		JWTSecret:      jwtSecret,
		AllowedOrigins: allowedOrigins,
		ReadTimeout:    getDuration("HTTP_READ_TIMEOUT", 10*time.Second),
		WriteTimeout:   getDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:    getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
	}, nil
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return def
}
