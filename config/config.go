package config

import (
	"os"
)

type Config struct {
	Port      string
	Database  string
	Frontend  string
	JWTSecret string
}

func Load() *Config {
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
		jwtSecret = "account-service-default-secret-change-in-production"
	}
	return &Config{
		Port:      port,
		Database:  dbPath,
		Frontend:  frontend,
		JWTSecret: jwtSecret,
	}
}
