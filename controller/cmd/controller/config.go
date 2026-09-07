package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	HTTPAddr            string
	DatabaseURL         string
	RedisURL            string
	RedisPassword       string
	AgentURL            string
	AgentCAFile         string
	AgentCertFile       string
	AgentKeyFile        string
	CookieSecure        bool
	TrustedOrigin       string
	SessionTTL          time.Duration
	PortStart           int
	PortEnd             int
	BindIP              string
	NodeMemoryMB        int
	NodeCPUs            int
	NodeDiskMB          int
	RegistrationEnabled bool
	ReconcileInterval   time.Duration
	SchedulerInterval   time.Duration
	BootstrapUsername   string
	BootstrapPassword   string
	CurseForgeAPIKey    string
}

func loadConfig() (config, error) {
	password, err := secret("DATABASE_PASSWORD", "DATABASE_PASSWORD_FILE")
	if err != nil {
		return config{}, err
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" && password != "" {
		databaseURL = fmt.Sprintf("postgres://mypanel:%s@postgres:5432/mypanel?sslmode=disable", url.QueryEscape(password))
	}
	redisPassword, err := secret("REDIS_PASSWORD", "REDIS_PASSWORD_FILE")
	if err != nil {
		return config{}, err
	}
	bootstrapPassword, err := secret("ADMIN_PASSWORD", "ADMIN_PASSWORD_FILE")
	if err != nil {
		return config{}, err
	}
	curseForgeAPIKey, err := secret("CURSEFORGE_API_KEY", "CURSEFORGE_API_KEY_FILE")
	if err != nil {
		return config{}, err
	}
	c := config{
		HTTPAddr:            env("HTTP_ADDR", ":8080"),
		DatabaseURL:         databaseURL,
		RedisURL:            env("REDIS_URL", "redis://redis:6379/0"),
		RedisPassword:       redisPassword,
		AgentURL:            env("AGENT_URL", "https://agent:8081"),
		AgentCAFile:         env("AGENT_CA_FILE", "/run/mypanel-certs/ca.crt"),
		AgentCertFile:       env("AGENT_CERT_FILE", "/run/mypanel-certs/controller.crt"),
		AgentKeyFile:        env("AGENT_KEY_FILE", "/run/mypanel-certs/controller.key"),
		CookieSecure:        envBool("COOKIE_SECURE", false),
		TrustedOrigin:       strings.TrimSuffix(os.Getenv("TRUSTED_ORIGIN"), "/"),
		SessionTTL:          8 * time.Hour,
		PortStart:           envInt("PORT_RANGE_START", 25565),
		PortEnd:             envInt("PORT_RANGE_END", 25749),
		BindIP:              env("GAME_BIND_IP", "0.0.0.0"),
		NodeMemoryMB:        envInt("NODE_MEMORY_MB", 12288),
		NodeCPUs:            envInt("NODE_CPUS", 7),
		NodeDiskMB:          envInt("NODE_DISK_MB", 102400),
		RegistrationEnabled: envBool("REGISTRATION_ENABLED", true),
		ReconcileInterval:   time.Duration(envInt("RECONCILE_SECONDS", 10)) * time.Second,
		SchedulerInterval:   time.Duration(envInt("SCHEDULER_SECONDS", 30)) * time.Second,
		BootstrapUsername:   env("ADMIN_USERNAME", "admin"),
		BootstrapPassword:   bootstrapPassword,
		CurseForgeAPIKey:    curseForgeAPIKey,
	}
	if c.DatabaseURL == "" {
		return config{}, fmt.Errorf("DATABASE_URL or DATABASE_PASSWORD_FILE is required")
	}
	if c.PortStart < 1 || c.PortEnd > 65535 || c.PortStart > c.PortEnd {
		return config{}, fmt.Errorf("invalid port range")
	}
	if c.NodeMemoryMB < 1 || c.NodeCPUs < 1 || c.NodeDiskMB < 1 {
		return config{}, fmt.Errorf("node capacity must be positive")
	}
	return c, nil
}

func secret(valueKey, fileKey string) (string, error) {
	if path := os.Getenv(fileKey); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", fileKey, err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return os.Getenv(valueKey), nil
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	return err == nil && b
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
