// Package httpapi serves the agent-facing HTTP surface: POST /reports and
// GET /healthz. The MCP endpoint (/mcp) mounts on the same mux.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/janrk/cairn/internal/intake"
	"github.com/janrk/cairn/internal/report"
)

// maxBody bounds a request body; it leaves room for all fields at their limits.
const maxBody = 256 * 1024

// Pinger reports database health.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Register adds the agent routes to mux.
func Register(mux *http.ServeMux, svc *intake.Service, db Pinger, log *slog.Logger) {
	mux.HandleFunc("POST /reports", postReport(svc, log))
	mux.HandleFunc("GET /healthz", Healthz(db))
}

// Healthz answers 200 when the database answers a ping.
func Healthz(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	}
}

type errorBody struct {
	Errors []report.FieldError `json:"errors"`
}

func postReport(svc *intake.Service, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in report.Input
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody{Errors: []report.FieldError{{Field: "body", Message: err.Error()}}})
			return
		}
		rc, err := svc.Submit(r.Context(), in, report.SourceREST)
		var ve *intake.ValidationError
		switch {
		case errors.As(err, &ve):
			writeJSON(w, http.StatusBadRequest, errorBody{Errors: ve.Fields})
		case err != nil:
			log.ErrorContext(r.Context(), "store report", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
		default:
			writeJSON(w, http.StatusCreated, rc)
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
