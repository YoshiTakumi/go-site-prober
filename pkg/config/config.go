package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	Port     string
	Targets  []string
	Interval time.Duration
	Timeout  time.Duration
}

func FromEnv() Config {
	return Config{
		Port:     getEnv("PORT", "8080"),
		Targets:  splitComma(os.Getenv("TARGETS")),
		Interval: parseDuration("INTERVAL", 15*time.Second),
		Timeout:  parseDuration("TIMEOUT", 5*time.Second),
	}
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
func parseDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
