package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/isyuricunha/YuFlix/services/core/internal/auth"
	"github.com/isyuricunha/YuFlix/services/core/internal/db"
	"github.com/isyuricunha/YuFlix/services/core/internal/httpapi"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := checkHealth(); err != nil {
			fmt.Fprintln(os.Stderr, "unhealthy:", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) > 1 && (os.Args[1] == "migrate" || os.Args[1] == "bootstrap-admin") {
		if err := runAdminCommand(); err != nil {
			fmt.Fprintln(os.Stderr, "operation failed:", err)
			os.Exit(1)
		}
		return
	}
	var store *db.DB
	var err error
	if dsn := os.Getenv("YUFLIX_DATABASE_URL"); dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		store, err = db.Open(ctx, dsn)
		if err != nil {
			// Never print database URLs or connection details.
			fmt.Fprintln(os.Stderr, "YuFlix database unavailable, private endpoints disabled")
		} else if !store.SchemaReady(ctx) {
			store.Close()
			store = nil
			fmt.Fprintln(os.Stderr, "YuFlix schema not migrated, private endpoints disabled")
		}
	}
	if store != nil {
		defer store.Close()
	}
	origin := os.Getenv("YUFLIX_PUBLIC_ORIGIN")
	allowedOrigins := strings.Split(os.Getenv("YUFLIX_ALLOWED_ORIGINS"), ",")
	allowHTTP := os.Getenv("YUFLIX_ALLOW_INSECURE_HTTP") == "true"
	trustProxy := os.Getenv("YUFLIX_TRUST_PROXY_HEADERS") == "true"
	addr := os.Getenv("YUFLIX_HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:62470"
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewRouter(httpapi.Options{Database: store, PublicOrigin: origin, AllowedOrigins: allowedOrigins, AllowInsecureHTTP: allowHTTP, TrustProxy: trustProxy}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    16 << 10,
		// Streaming routes need independent request-level timeouts.
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("YuFlix Core starting", "address", addr)
		errCh <- server.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown failed", "error", err)
			os.Exit(1)
		}
		logger.Info("YuFlix Core stopped")
	}
}

func checkHealth() error {
	client := http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy:       nil,
			DialContext: (&net.Dialer{Timeout: time.Second}).DialContext,
		},
	}
	resp, err := client.Get("http://127.0.0.1:62470/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func runAdminCommand() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("YUFLIX_DATABASE_URL")
	d, err := db.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer d.Close()
	switch os.Args[1] {
	case "migrate":
		if err = d.Migrate(ctx); err != nil {
			return err
		}
		fmt.Println("YuFlix schema migrated")
		return nil
	case "bootstrap-admin":
		if len(os.Args) != 3 {
			return fmt.Errorf("usage: yuflix bootstrap-admin <username>")
		}
		username := strings.ToLower(strings.TrimSpace(os.Args[2]))
		if !auth.ValidUsername(username) {
			return fmt.Errorf("invalid username")
		}
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return fmt.Errorf("interactive terminal required")
		}
		fmt.Fprint(os.Stderr, "New admin password: ")
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stderr, "Confirm admin password: ")
		again, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		if string(password) != string(again) {
			return fmt.Errorf("passwords differ")
		}
		hash, err := auth.HashPassword(string(password))
		if err != nil {
			return err
		}
		u, err := d.CreateFirstAdmin(ctx, username, hash)
		if err != nil {
			return err
		}
		fmt.Printf("Bootstrap administrator created: %s\n", u.Username)
		return nil
	default:
		return fmt.Errorf("unknown command")
	}
}
