package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janrk/cairn/internal/reviewmcp"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	if code := run(context.Background(), []string{"--version"}, func(string) string { return "" }, &out, &out); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.HasPrefix(out.String(), "cairn ") {
		t.Fatalf("out %q", out.String())
	}
}

func TestServeHealthcheckAndLogs(t *testing.T) {
	addr := freeAddr(t)
	env := map[string]string{
		"CAIRN_DB_PATH":     filepath.Join(t.TempDir(), "cairn.db"),
		"CAIRN_ADDR":        addr,
		"CAIRN_REVIEW_ADDR": freeAddr(t),
	}
	getenv := func(k string) string { return env[k] }

	// Nothing listening yet: healthcheck must fail.
	var errOut bytes.Buffer
	if code := run(context.Background(), []string{"healthcheck"}, getenv, &errOut, &errOut); code != 1 {
		t.Fatalf("healthcheck on dead server: exit %d", code)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var logs syncBuf
	done := make(chan int)
	go func() { done <- run(ctx, nil, getenv, &logs, &logs) }()

	deadline := time.Now().Add(5 * time.Second)
	for run(context.Background(), []string{"healthcheck"}, getenv, &errOut, &errOut) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("server never healthy: %s", logs.String())
		}
		time.Sleep(20 * time.Millisecond)
	}

	resp, err := http.Post("http://"+addr+"/reports", "application/json",
		strings.NewReader(`{"kind":"outage","summary":"down","details":"TOPSECRETDETAIL","subject":"litellm"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", resp.StatusCode)
	}

	for _, c := range []struct {
		addr string
		want int
	}{
		{env["CAIRN_REVIEW_ADDR"] + reviewmcp.Path, http.StatusOK},
		{addr + reviewmcp.Path, http.StatusNotFound},
	} {
		resp, err := http.Post("http://"+c.addr, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound != (c.want == http.StatusNotFound) {
			t.Fatalf("%s: status %d", c.addr, resp.StatusCode)
		}
	}

	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("serve exit %d: %s", code, logs.String())
	}

	var found bool
	for line := range strings.SplitSeq(strings.TrimSpace(logs.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("non-JSON log line %q", line)
		}
		if m["msg"] == "report stored" {
			found = m["id"] != "" && m["kind"] == "outage" && m["subject"] == "litellm"
		}
	}
	if !found {
		t.Fatalf("no report log line: %s", logs.String())
	}
	if strings.Contains(logs.String(), "TOPSECRETDETAIL") {
		t.Fatal("details logged")
	}
}
