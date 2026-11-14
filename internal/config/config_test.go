package config

import (
	"log/slog"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaults(t *testing.T) {
	c, err := FromEnv(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{DBPath: "/data/cairn.db", Addr: ":8080", ReviewAddr: ":8081", Retention: 168 * time.Hour, LogLevel: slog.LevelInfo}
	if c != want {
		t.Fatalf("got %+v", c)
	}
}

func TestOverridesAndErrors(t *testing.T) {
	c, err := FromEnv(env(map[string]string{"CAIRN_DB_PATH": "/x.db", "CAIRN_RETENTION": "1h", "CAIRN_LOG_LEVEL": "debug"}))
	if err != nil || c.DBPath != "/x.db" || c.Retention != time.Hour || c.LogLevel != slog.LevelDebug {
		t.Fatalf("got %+v %v", c, err)
	}
	for _, m := range []map[string]string{{"CAIRN_RETENTION": "soon"}, {"CAIRN_RETENTION": "-1h"}, {"CAIRN_LOG_LEVEL": "loud"}} {
		if _, err := FromEnv(env(m)); err == nil {
			t.Errorf("%v accepted", m)
		}
	}
}
