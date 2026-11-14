// Package config reads cairn's environment configuration.
package config

import (
	"fmt"
	"log/slog"
	"time"
)

// Config is cairn's runtime configuration.
type Config struct {
	DBPath     string
	Addr       string
	ReviewAddr string
	Retention  time.Duration
	LogLevel   slog.Level
}

// FromEnv builds a Config from getenv, applying defaults.
func FromEnv(getenv func(string) string) (Config, error) {
	get := func(k, def string) string {
		if v := getenv(k); v != "" {
			return v
		}
		return def
	}
	c := Config{
		DBPath:     get("CAIRN_DB_PATH", "/data/cairn.db"),
		Addr:       get("CAIRN_ADDR", ":8080"),
		ReviewAddr: get("CAIRN_REVIEW_ADDR", ":8081"),
	}
	var err error
	if c.Retention, err = time.ParseDuration(get("CAIRN_RETENTION", "168h")); err != nil {
		return Config{}, fmt.Errorf("CAIRN_RETENTION: %w", err)
	}
	if c.Retention <= 0 {
		return Config{}, fmt.Errorf("CAIRN_RETENTION: must be positive")
	}
	if err := c.LogLevel.UnmarshalText([]byte(get("CAIRN_LOG_LEVEL", "info"))); err != nil {
		return Config{}, fmt.Errorf("CAIRN_LOG_LEVEL: %w", err)
	}
	return c, nil
}
