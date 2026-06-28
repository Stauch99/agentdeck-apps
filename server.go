/**
 * [INPUT]: Depends on encoding/json, net/http, io/fs, path, strings from stdlib; Service+Library from this module
 * [OUTPUT]: Provides Server, NewServer, (*Server).Handler — the openmusic REST + static surface
 * [POS]: HTTP boundary of openmusic; consumed by main.go, drives Service and reads Library
 * [PROTOCOL]: When changing, update this header, then check openmusic/CLAUDE.md
 */
package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

type Server struct {
	svc *Service
	lib *Library
	web fs.FS
}

func NewServer(svc *Service, lib *Library, web fs.FS) *Server {
	return &Server{svc: svc, lib: lib, web: web}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("/api/generate", s.handleGenerate)
	mux.HandleFunc("/api/songs", s.handleSongs)
	mux.HandleFunc("/api/songs/", s.handleSongDelete)
	mux.HandleFunc("/media/", s.handleMedia)
	mux.Handle("/", http.FileServer(http.FS(s.web)))
	return mux
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	taskID, err := s.svc.Submit(ctx, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"taskId": taskID})
}

func (s *Server) handleSongs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"songs": s.lib.List()})
}

func (s *Server) handleSongDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "DELETE only"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/songs/")
	s.lib.Delete(id)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "deleted"})
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/media/")
	ext := strings.TrimPrefix(path.Ext(name), ".")
	id := strings.TrimSuffix(name, "."+ext)
	p := s.lib.MediaPath(id, ext)
	if p == "" {
		http.Error(w, "bad media id", http.StatusBadRequest)
		return
	}
	http.ServeFile(w, r, p)
}
