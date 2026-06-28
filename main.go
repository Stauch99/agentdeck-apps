/**
 * [INPUT]: Depends on net/http, os from stdlib
 * [OUTPUT]: Provides the OpenMusic process entrypoint (main) + env helpers
 * [POS]: Assembly root of the openmusic module; wires Library + Service + Server (full wiring in Task 6)
 * [PROTOCOL]: When changing, update this header, then check openmusic/CLAUDE.md
 */
package main

import (
	"log"
	"net/http"
	"os"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := envOr("OPENMUSIC_ADDR", ":8080")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	log.Printf("OpenMusic listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
