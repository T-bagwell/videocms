package config

import (
	"log"
	"os"
)

type Config struct {
	Addr          string
	DatabaseURL   string
	JWTSecret     string
	DataDir       string
	AdminUsername string
	AdminPassword string
	TMDBAPIKey    string
	WebRoot       string
}

func Load() Config {
	cfg := Config{
		Addr:          envOr("PORT", "8080"),
		DatabaseURL:   envOr("DATABASE_URL", "postgres://localhost:5432/videocms?sslmode=disable"),
		JWTSecret:     envOr("JWT_SECRET", "videocms-dev-secret-change-me"),
		DataDir:       envOr("DATA_DIR", "data"),
		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		TMDBAPIKey:    os.Getenv("TMDB_API_KEY"),
		WebRoot:       os.Getenv("WEB_ROOT"),
	}
	if cfg.Addr != "" && cfg.Addr[0] != ':' {
		cfg.Addr = ":" + cfg.Addr
	}
	if cfg.JWTSecret == "videocms-dev-secret-change-me" {
		log.Println("WARNING: using default JWT_SECRET; set JWT_SECRET in production")
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
