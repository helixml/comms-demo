package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/helixml/comms-demo/helixclient"
	"github.com/helixml/comms-demo/server"
	"github.com/helixml/comms-demo/store"
	"github.com/helixml/comms-demo/web"
)

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", envOr("MOCK_CHANNELS_ADDR", ":7765"), "listen address")
	dbPath := fs.String("db", envOr("MOCK_CHANNELS_DB", "mock-channels.db"), "sqlite db path")
	logFmt := fs.String("log", envOr("MOCK_CHANNELS_LOG", "text"), "log format: text|json")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("serve: parse flags: %w", err)
	}

	logger := newLogger(*logFmt)

	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	srv, err := server.New(server.Deps{
		Store:       st,
		Helix:       helixclient.New(nil),
		Logger:      logger,
		TemplatesFS: web.FS,
		IDGen:       server.HexIDGen{},
		Clock:       server.SystemClock{},
	})
	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", *addr, "db", *dbPath)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http listen: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
	case err := <-errCh:
		if err != nil {
			return err
		}
	}
	return nil
}

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func newLogger(format string) *slog.Logger {
	switch strings.ToLower(format) {
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	default:
		return slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
}
