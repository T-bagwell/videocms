package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr                   string
	DatabaseURL            string
	JWTSecret              string
	DataDir                string
	AdminUsername          string
	AdminPassword          string
	TMDBAPIKey             string
	WebRoot                string
	WatchInterval          time.Duration
	YtDLPPath              string
	CORSOrigins            []string
	HLSHWAccel             string
	HLSVAAPIDevice         string
	HLSToneMap             bool
	HLSVCodec              string
	HLSPassthroughHDR      bool
	SubtitleOSUser         string
	SubtitleOSPassword     string
	SubtitleOSAPIKey       string
	RTMPIngestURL          string
	WhisperBin             string
	WhisperModel           string
	RegistrationEnabled    bool
	RegistrationInviteOnly bool
	ScrapeCustomURL        string
	AITagBin               string
	OIDCIssuer             string
	OIDCClientID           string
	OIDCClientSecret       string
	OIDCRedirectURL        string
	SAMLIDPMetadataURL     string
	SAMLSPCert             string
	SAMLSPKey              string
	SAMLSPEntityID         string
	SAMLACSURL             string
	NotifyWebhookURL       string
	NotifyAppriseURL       string
	SMTPHost               string
	SMTPPort               int
	SMTPUser               string
	SMTPPassword           string
	NotifyEmailFrom        string
	NotifyEmailTo          []string
	MaintIntervalHours     int
	MaintRetention         int
	MaintRescan            bool
	DLNAEnabled            bool
	DLNAFriendlyName       string
	DLNAAllowedIPs         []string
}

func Load() Config {
	cfg := Config{
		Addr:                   envOr("PORT", "8080"),
		DatabaseURL:            envOr("DATABASE_URL", "postgres://localhost:5432/videocms?sslmode=disable"),
		JWTSecret:              envOr("JWT_SECRET", "videocms-dev-secret-change-me"),
		DataDir:                envOr("DATA_DIR", "data"),
		AdminUsername:          os.Getenv("ADMIN_USERNAME"),
		AdminPassword:          os.Getenv("ADMIN_PASSWORD"),
		TMDBAPIKey:             os.Getenv("TMDB_API_KEY"),
		WebRoot:                os.Getenv("WEB_ROOT"),
		WatchInterval:          envDuration("WATCH_INTERVAL", 30*time.Second),
		YtDLPPath:              os.Getenv("YTDLP_PATH"),
		CORSOrigins:            envList("CORS_ORIGINS"),
		HLSHWAccel:             os.Getenv("HLS_HW_ACCEL"),
		HLSVAAPIDevice:         os.Getenv("HLS_VAAPI_DEVICE"),
		HLSToneMap:             envBool("HLS_TONE_MAP"),
		HLSVCodec:              os.Getenv("HLS_VCODEC"),
		HLSPassthroughHDR:      os.Getenv("HLS_PASSTHROUGH_HDR") != "0",
		SubtitleOSUser:         os.Getenv("SUBTITLE_OS_USERNAME"),
		SubtitleOSPassword:     os.Getenv("SUBTITLE_OS_PASSWORD"),
		SubtitleOSAPIKey:       os.Getenv("SUBTITLE_OS_API_KEY"),
		RTMPIngestURL:          envOr("RTMP_INGEST_URL", "rtmp://localhost:1935/live"),
		WhisperBin:             os.Getenv("WHISPER_BIN"),
		WhisperModel:           os.Getenv("WHISPER_MODEL"),
		RegistrationEnabled:    os.Getenv("REGISTRATION_ENABLED") != "0",
		RegistrationInviteOnly: os.Getenv("REGISTRATION_INVITE_ONLY") == "1",
		ScrapeCustomURL:        os.Getenv("SCRAPE_CUSTOM_URL"),
		AITagBin:               os.Getenv("AI_TAG_BIN"),
		OIDCIssuer:             os.Getenv("OIDC_ISSUER"),
		OIDCClientID:           os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret:       os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:        os.Getenv("OIDC_REDIRECT_URL"),
		SAMLIDPMetadataURL:     os.Getenv("SAML_IDP_METADATA_URL"),
		SAMLSPCert:             os.Getenv("SAML_SP_CERT"),
		SAMLSPKey:              os.Getenv("SAML_SP_KEY"),
		SAMLSPEntityID:         os.Getenv("SAML_SP_ENTITY_ID"),
		SAMLACSURL:             os.Getenv("SAML_ACS_URL"),
		NotifyWebhookURL:       os.Getenv("NOTIFY_WEBHOOK_URL"),
		NotifyAppriseURL:       os.Getenv("NOTIFY_APPRISE_URL"),
		SMTPHost:               os.Getenv("SMTP_HOST"),
		SMTPPort:               envInt("SMTP_PORT", 587),
		SMTPUser:               os.Getenv("SMTP_USER"),
		SMTPPassword:           os.Getenv("SMTP_PASSWORD"),
		NotifyEmailFrom:        os.Getenv("NOTIFY_EMAIL_FROM"),
		NotifyEmailTo:          envList("NOTIFY_EMAIL_TO"),
		MaintIntervalHours:     envInt("MAINT_INTERVAL_HOURS", 24),
		MaintRetention:         envInt("MAINT_BACKUP_RETENTION", 7),
		MaintRescan:            envBool("MAINT_RESCAN"),
		DLNAEnabled:            envBool("DLNA_ENABLED"),
		DLNAFriendlyName:       os.Getenv("DLNA_FRIENDLY_NAME"),
		DLNAAllowedIPs:         envList("DLNA_ALLOWED_IPS"),
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

func envInt(key string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return fallback
}
