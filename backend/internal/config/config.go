package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr               string
	DatabaseURL        string
	JWTSecret          string
	DataDir            string
	AdminUsername      string
	AdminPassword      string
	TMDBAPIKey         string
	WebRoot            string
	WatchInterval      time.Duration
	YtDLPPath          string
	CORSOrigins        []string
	HLSHWAccel         string
	HLSVAAPIDevice     string
	HLSToneMap         bool
	SubtitleOSUser     string
	SubtitleOSPassword string
	SubtitleOSAPIKey   string
}

func Load() Config {
	cfg := Config{
		Addr:               envOr("PORT", "8080"),
		DatabaseURL:        envOr("DATABASE_URL", "postgres://localhost:5432/videocms?sslmode=disable"),
		JWTSecret:          envOr("JWT_SECRET", "videocms-dev-secret-change-me"),
		DataDir:            envOr("DATA_DIR", "data"),
		AdminUsername:      os.Getenv("ADMIN_USERNAME"),
		AdminPassword:      os.Getenv("ADMIN_PASSWORD"),
		TMDBAPIKey:         os.Getenv("TMDB_API_KEY"),
		WebRoot:            os.Getenv("WEB_ROOT"),
		WatchInterval:      envDuration("WATCH_INTERVAL", 30*time.Second),
		YtDLPPath:          os.Getenv("YTDLP_PATH"),
		CORSOrigins:        envList("CORS_ORIGINS"),
		HLSHWAccel:         os.Getenv("HLS_HW_ACCEL"),
		HLSVAAPIDevice:     os.Getenv("HLS_VAAPI_DEVICE"),
		HLSToneMap:         envBool("HLS_TONE_MAP"),
		SubtitleOSUser:     os.Getenv("SUBTITLE_OS_USERNAME"),
		SubtitleOSPassword: os.Getenv("SUBTITLE_OS_PASSWORD"),
		SubtitleOSAPIKey:   os.Getenv("SUBTITLE_OS_API_KEY"),
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

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return fallback
}

// envList splits a comma-separated env value into trimmed non-empty items.
func envList(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
