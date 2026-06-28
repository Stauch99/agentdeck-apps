/**
 * [INPUT]: Depends on context, embed, io/fs, log, net/http, os, os/signal, syscall, time from stdlib; Library/Client/Service/Server from this module
 * [OUTPUT]: Provides the OpenMusic process entrypoint (main)
 * [POS]: Assembly root of openmusic; reads env, embeds web/, wires the stack, serves with graceful shutdown
 * [PROTOCOL]: When changing, update this header, then check openmusic/CLAUDE.md
 */
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed web
var webFS embed.FS

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	apiKey := os.Getenv("KIE_API_KEY")
	if apiKey == "" {
		log.Fatal("KIE_API_KEY not set")
	}
	baseURL := envOr("KIE_BASE_URL", "https://api.kie.ai")
	dataDir := envOr("OPENMUSIC_DATA_DIR", "/data")
	addr := envOr("OPENMUSIC_ADDR", ":8080")

	// Process-lifetime context, cancelled on SIGINT/SIGTERM, drives graceful shutdown + stops poll loops.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lib, err := NewLibrary(dataDir)
	if err != nil {
		log.Fatalf("library: %v", err)
	}
	web, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("embed web: %v", err)
	}
	client := NewClient(baseURL, apiKey)
	if cb := os.Getenv("KIE_CALLBACK_URL"); cb != "" {
		client.callbackURL = cb // optional: a real public callback endpoint if you ever expose one; we poll regardless
	}
	svc := NewService(client, lib)
	svc.baseCtx = ctx
	// Dev/test escape hatch: permit media downloads from loopback/private hosts (e.g. the fake-kie harness).
	// Unset in production so the SSRF guard blocks internal targets; real kie media lives on public CDNs.
	if envOr("OPENMUSIC_ALLOW_PRIVATE_MEDIA", "") != "" {
		svc.blockPrivateHosts = false
	}
	srv := NewServer(svc, lib, web)

	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler()}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpSrv.Shutdown(sctx)
	}()

	log.Printf("OpenMusic on %s (kie=%s data=%s)", addr, baseURL, dataDir)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
