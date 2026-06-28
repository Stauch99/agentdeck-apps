package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func newTestServer(t *testing.T) (*Server, *Library) {
	t.Helper()
	lib, _ := NewLibrary(t.TempDir())
	// kie stub: generate returns a taskId, record-info immediately PENDING (poll won't finish in test)
	kie := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "record-info") {
			w.Write([]byte(`{"code":200,"msg":"ok","data":{"status":"PENDING","response":{"sunoData":[]}}}`))
			return
		}
		w.Write([]byte(`{"code":200,"msg":"ok","data":{"taskId":"TT"}}`))
	}))
	t.Cleanup(kie.Close)
	svc := NewService(NewClient(kie.URL, "k"), lib)
	svc.interval = time.Hour // don't actually poll during handler tests
	web := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<h1>OpenMusic</h1>")}}
	return NewServer(svc, lib, web), lib
}

func TestGenerateHandlerValidates(t *testing.T) {
	srv, _ := newTestServer(t)
	// missing prompt in simple mode → 400
	r := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{"customMode":false,"model":"V4"}`))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestGenerateHandlerAcceptsValid(t *testing.T) {
	srv, lib := newTestServer(t)
	r := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{"customMode":false,"model":"V4","prompt":"hello"}`))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body)
	}
	var resp struct {
		TaskID string `json:"taskId"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.TaskID != "TT" {
		t.Fatalf("taskId = %q", resp.TaskID)
	}
	if len(lib.List()) != 2 {
		t.Fatalf("placeholders not created")
	}
}

func TestSongsAndMediaAndStatic(t *testing.T) {
	srv, lib := newTestServer(t)
	lib.AddPlaceholders("T1", "V4", "pop", "Song", 1)
	lib.Materialize("T1", 0, Track{ID: "aud9", Title: "Song"})
	lib.SaveMedia("aud9", "mp3", []byte("AUDIO"))
	lib.MarkDone("aud9", true, false)

	// GET /api/songs
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/songs", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "aud9") {
		t.Fatalf("songs: %d %s", w.Code, w.Body)
	}
	// GET /media/aud9.mp3
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/media/aud9.mp3", nil))
	if w.Code != 200 || w.Body.String() != "AUDIO" {
		t.Fatalf("media: %d %q", w.Code, w.Body)
	}
	// path traversal rejected
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/media/..%2f..%2flibrary.json", nil))
	if w.Code == 200 {
		t.Fatalf("traversal should not 200")
	}
	// static index
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "OpenMusic") {
		t.Fatalf("static: %d %s", w.Code, w.Body)
	}
	_ = context.Background()
}
