// Command cairn is the field-notes inbox for AI agents.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/janrk/cairn/internal/agentmcp"
	"github.com/janrk/cairn/internal/config"
	"github.com/janrk/cairn/internal/httpapi"
	"github.com/janrk/cairn/internal/intake"
	"github.com/janrk/cairn/internal/reviewmcp"
	"github.com/janrk/cairn/internal/store"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cairn", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		_, _ = fmt.Fprintln(stdout, "cairn", version)
		return 0
	}

	cfg, err := config.FromEnv(getenv)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "config:", err)
		return 2
	}

	switch fs.Arg(0) {
	case "":
		log := slog.New(slog.NewJSONHandler(stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
		if err := serve(ctx, cfg, log); err != nil {
			log.Error("cairn stopped", "err", err)
			return 1
		}
		return 0
	case "healthcheck":
		if err := healthcheck(ctx, cfg.Addr); err != nil {
			_, _ = fmt.Fprintln(stderr, "unhealthy:", err)
			return 1
		}
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n", fs.Arg(0))
		return 2
	}
}

func serve(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	agentMux := http.NewServeMux()
	svc := intake.New(st, log)
	httpapi.Register(agentMux, svc, st, log)
	agentmcp.Register(agentMux, svc, log)

	reviewMux := http.NewServeMux()
	reviewmcp.Register(reviewMux, &reviewmcp.Service{Store: st, Now: time.Now})
	reviewMux.HandleFunc("GET /healthz", httpapi.Healthz(st))

	servers := []*http.Server{
		{Addr: cfg.Addr, Handler: agentMux, ReadHeaderTimeout: 10 * time.Second},
		{Addr: cfg.ReviewAddr, Handler: reviewMux, ReadHeaderTimeout: 10 * time.Second},
	}

	errc := make(chan error, len(servers))
	for _, srv := range servers {
		log.Info("listening", "addr", srv.Addr, "version", version)
		go func() {
			if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				errc <- err
			}
		}()
	}

	select {
	case <-ctx.Done():
	case err = <-errc:
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	for _, srv := range servers {
		_ = srv.Shutdown(shutdownCtx)
	}
	return err
}

func healthcheck(ctx context.Context, addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("CAIRN_ADDR: %w", err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %s", strings.TrimSpace(resp.Status))
	}
	return nil
}
