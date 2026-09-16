// Command request-logger accepts any HTTP request on any route and writes a
// structured record of it to stdout via slog.
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	addr := envOr("ADDR", ":8080")
	status, err := strconv.Atoi(envOr("RESPONSE_STATUS", "200"))
	if err != nil || status < 100 || status > 599 {
		slog.Error("invalid RESPONSE_STATUS", "value", os.Getenv("RESPONSE_STATUS"))
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           &handler{status: status},
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("listening", "addr", addr, "response_status", status)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "error", err)
	}
}

type handler struct {
	status int
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	body, readErr := io.ReadAll(r.Body)

	attrs := make([]any, 0, 14)
	attrs = append(attrs,
		slog.String("method", r.Method),
		slog.String("host", r.Host),
		slog.String("path", r.URL.Path),
		slog.String("proto", r.Proto),
		slog.String("remote_addr", r.RemoteAddr),
	)
	if r.URL.RawQuery != "" {
		attrs = append(attrs, slog.String("query", r.URL.RawQuery))
	}
	attrs = append(attrs, slog.Any("headers", headerAttrs(r.Header)))
	if len(r.TransferEncoding) > 0 {
		attrs = append(attrs, slog.String("transfer_encoding", strings.Join(r.TransferEncoding, ",")))
	}
	if r.TLS != nil {
		attrs = append(attrs, slog.Group("tls",
			slog.String("version", tlsVersion(r.TLS.Version)),
			slog.String("server_name", r.TLS.ServerName),
		))
	}

	attrs = append(attrs, slog.Int("body_bytes", len(body)))
	if readErr != nil {
		attrs = append(attrs, slog.String("body_read_error", readErr.Error()))
	}
	if len(body) > 0 {
		// Binary payloads would otherwise be mangled into replacement characters.
		if utf8.Valid(body) {
			attrs = append(attrs, slog.String("body", string(body)))
		} else {
			attrs = append(attrs,
				slog.String("body_b64", base64.StdEncoding.EncodeToString(body)),
				slog.String("body_encoding", "base64"),
			)
		}
	}

	attrs = append(attrs, slog.Duration("duration_ns", time.Since(start)))
	slog.Info("request", attrs...)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.status)
	_, _ = io.WriteString(w, `{"logged":true}`+"\n")
}

// headerAttrs keeps repeated headers intact: a single value logs as a string,
// repeats log as an array.
func headerAttrs(h http.Header) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(h))
	for name, values := range h {
		if len(values) == 1 {
			attrs = append(attrs, slog.String(name, values[0]))
			continue
		}
		attrs = append(attrs, slog.Any(name, values))
	}
	return attrs
}

func tlsVersion(v uint16) string {
	switch v {
	case 0x0301:
		return "1.0"
	case 0x0302:
		return "1.1"
	case 0x0303:
		return "1.2"
	case 0x0304:
		return "1.3"
	default:
		return "unknown"
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
