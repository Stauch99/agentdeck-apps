/**
 * [INPUT]: Depends on embed, io/fs, log, net/http, os from stdlib; Library/Client/Service/Server from this module
 * [OUTPUT]: Provides the OpenMusic process entrypoint (main)
 * [POS]: Assembly root of openmusic; reads env, embeds web/, wires the stack, serves
 * [PROTOCOL]: When changing, update this header, then check openmusic/CLAUDE.md
 */
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
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

	lib, err := NewLibrary(dataDir)
	if err != nil {
		log.Fatalf("library: %v", err)
	}
	web, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("embed web: %v", err)
	}
	svc := NewService(NewClient(baseURL, apiKey), lib)
	srv := NewServer(svc, lib, web)

	log.Printf("OpenMusic on %s (kie=%s data=%s)", addr, baseURL, dataDir)
	log.Fatal(http.ListenAndServe(addr, srv.Handler()))
}
