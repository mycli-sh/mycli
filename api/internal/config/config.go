package config

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL       string
	Port              string
	JWTSecret         string
	BaseURL           string
	ResendAPIKey      string
	EmailFrom         string
	AllowedOrigins    []string
	WebBaseURL        string
	SystemAdminEmails []string
}

func Load() *Config {
	cfg := &Config{
		DatabaseURL:       buildDatabaseURL(),
		Port:              getEnv("PORT", "8080"),
		JWTSecret:         getEnv("JWT_SECRET", "dev-secret-change-me"),
		BaseURL:           getEnv("BASE_URL", "http://localhost:8080"),
		ResendAPIKey:      getEnv("RESEND_API_KEY", ""),
		EmailFrom:         getEnv("EMAIL_FROM", "mycli@updates.mycli.sh"),
		AllowedOrigins:    parseOrigins(getEnv("ALLOWED_ORIGINS", "http://localhost:5173")),
		WebBaseURL:        getEnv("WEB_BASE_URL", "http://localhost:5173"),
		SystemAdminEmails: parseEmails(getEnv("SYSTEM_ADMIN_EMAILS", "")),
	}

	// Refuse to start with the default JWT secret in non-localhost deployments
	if cfg.JWTSecret == "dev-secret-change-me" && !isLocalhost(cfg.BaseURL) {
		log.Fatal("FATAL: JWT_SECRET must be set to a secure value in non-localhost deployments")
	}

	return cfg
}

func isLocalhost(baseURL string) bool {
	lower := strings.ToLower(baseURL)
	return strings.Contains(lower, "://localhost") || strings.Contains(lower, "://127.0.0.1")
}

// buildDatabaseURL returns a database connection string. If DATABASE_URL is set,
// it is used as-is. Otherwise, if any individual DB_* vars are set, a DSN is
// constructed from them. If neither is provided, the default local dev DSN is used.
func buildDatabaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}

	const defaultDSN = "postgres://mycli:mycli@localhost:5432/mycli_dev?sslmode=disable"

	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	port := os.Getenv("DB_PORT")
	name := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	// If no individual vars are set, use the default.
	if host == "" && user == "" && password == "" && port == "" && name == "" && sslmode == "" {
		return defaultDSN
	}

	// Apply defaults for any unset individual vars.
	if host == "" {
		host = "localhost"
	}
	if user == "" {
		user = "mycli"
	}
	if password == "" {
		password = "mycli"
	}
	if port == "" {
		port = "5432"
	}
	if name == "" {
		name = "mycli_dev"
	}
	if sslmode == "" {
		sslmode = "disable"
	}

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, password, host, port, name, sslmode)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// IsSystemAdmin checks whether the given email is in the system admin list.
func (c *Config) IsSystemAdmin(email string) bool {
	lower := strings.ToLower(strings.TrimSpace(email))
	for _, e := range c.SystemAdminEmails {
		if e == lower {
			return true
		}
	}
	return false
}

func parseEmails(s string) []string {
	var emails []string
	for _, e := range strings.Split(s, ",") {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			emails = append(emails, e)
		}
	}
	return emails
}

func parseOrigins(s string) []string {
	var origins []string
	for _, o := range strings.Split(s, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}
