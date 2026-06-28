package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateParsesTaskID(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/generate" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer k123" {
			t.Errorf("auth header = %q", got)
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &gotBody)
		w.Write([]byte(`{"code":200,"msg":"success","data":{"taskId":"T-1"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k123")
	id, err := c.Generate(context.Background(), GenerateRequest{
		CustomMode: true, Model: "V4_5ALL", Prompt: "la la", Style: "pop", Title: "Hi",
	})
	if err != nil || id != "T-1" {
		t.Fatalf("Generate => %q, %v", id, err)
	}
	// callBackUrl must NOT be sent (pure polling); customMode/model must be present.
	if _, ok := gotBody["callBackUrl"]; ok {
		t.Errorf("callBackUrl should be omitted")
	}
	if gotBody["model"] != "V4_5ALL" || gotBody["customMode"] != true {
		t.Errorf("body missing fields: %v", gotBody)
	}
}

func TestGenerateAPIErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":401,"msg":"unauthorized","data":null}`))
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL, "bad").Generate(context.Background(), GenerateRequest{CustomMode: false, Prompt: "x", Model: "V4"})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("want 401 error, got %v", err)
	}
}

func TestRecordInfoParsesTracks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("taskId") != "T-1" {
			t.Errorf("taskId = %q", r.URL.Query().Get("taskId"))
		}
		w.Write([]byte(`{"code":200,"msg":"success","data":{"status":"SUCCESS","response":{"sunoData":[
			{"id":"a0","audioUrl":"http://x/a0.mp3","imageUrl":"http://x/a0.jpg","title":"One","tags":"pop","duration":120},
			{"id":"a1","audioUrl":"http://x/a1.mp3","imageUrl":"http://x/a1.jpg","title":"Two","tags":"pop","duration":121}
		]}}}`))
	}))
	defer srv.Close()
	status, tracks, errMsg, err := NewClient(srv.URL, "k").RecordInfo(context.Background(), "T-1")
	if err != nil || status != "SUCCESS" || errMsg != "" || len(tracks) != 2 {
		t.Fatalf("RecordInfo => %s %d %q %v", status, len(tracks), errMsg, err)
	}
	if tracks[0].ID != "a0" || tracks[1].AudioURL != "http://x/a1.mp3" {
		t.Fatalf("bad tracks: %+v", tracks)
	}
}

func TestValidateCustomModeRequiresFields(t *testing.T) {
	cases := []struct {
		name string
		req  GenerateRequest
		ok   bool
	}{
		{"custom+vocal needs all", GenerateRequest{CustomMode: true, Model: "V4", Prompt: "p", Style: "s", Title: "t"}, true},
		{"custom missing title", GenerateRequest{CustomMode: true, Model: "V4", Prompt: "p", Style: "s"}, false},
		{"custom instrumental needs style+title", GenerateRequest{CustomMode: true, Instrumental: true, Model: "V4", Style: "s", Title: "t"}, true},
		{"simple needs prompt", GenerateRequest{CustomMode: false, Model: "V4"}, false},
		{"simple ok", GenerateRequest{CustomMode: false, Model: "V4", Prompt: "p"}, true},
		{"bad model", GenerateRequest{CustomMode: false, Model: "V9", Prompt: "p"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.req.Validate(); (err == nil) != tc.ok {
				t.Fatalf("Validate ok=%v, err=%v", tc.ok, err)
			}
		})
	}
}

func TestServiceSubmitPollsAndCachesMedia(t *testing.T) {
	calls := 0
	var base string // kie stub points generated media URLs back at its own server; set after it starts
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/generate") && r.Method == http.MethodPost:
			w.Write([]byte(`{"code":200,"msg":"ok","data":{"taskId":"T9"}}`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/generate/record-info"):
			calls++
			if calls == 1 { // first poll: still pending
				w.Write([]byte(`{"code":200,"msg":"ok","data":{"status":"PENDING","response":{"sunoData":[]}}}`))
				return
			}
			// second poll: success with two tracks pointing back at this server for media
			fmt.Fprintf(w, `{"code":200,"msg":"ok","data":{"status":"SUCCESS","response":{"sunoData":[
				{"id":"a0","audioUrl":"%s/m/a0.mp3","imageUrl":"%s/m/a0.jpg","title":"One","tags":"pop","duration":120},
				{"id":"a1","audioUrl":"%s/m/a1.mp3","imageUrl":"%s/m/a1.jpg","title":"Two","tags":"pop","duration":121}
			]}}}`, base, base, base, base)
		case strings.HasPrefix(r.URL.Path, "/m/"):
			w.Write([]byte("BINARY:" + r.URL.Path))
		}
	}))
	defer srv.Close()
	base = srv.URL // media URLs point back at this server (loopback)

	dir := t.TempDir()
	lib, _ := NewLibrary(dir)
	svc := NewService(NewClient(srv.URL, "k"), lib)
	svc.interval = 5 * time.Millisecond // fast polling for the test
	svc.timeout = 2 * time.Second
	svc.blockPrivateHosts = false // media downloads come from the loopback httptest server

	taskID, err := svc.Submit(context.Background(), GenerateRequest{CustomMode: false, Model: "V4", Prompt: "hi"})
	if err != nil || taskID != "T9" {
		t.Fatalf("Submit => %q %v", taskID, err)
	}
	// two placeholders appear immediately
	if len(lib.List()) != 2 {
		t.Fatalf("want 2 placeholders, got %d", len(lib.List()))
	}
	// wait for the poll goroutine to finish both tracks
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		done := 0
		for _, s := range lib.List() {
			if s.Status == "done" {
				done++
			}
		}
		if done == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, s := range lib.List() {
		if s.Status != "done" || !s.HasAudio || !s.HasCover {
			t.Fatalf("song not finished: %+v", s)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "media", "a0.mp3")); !strings.HasPrefix(string(b), "BINARY:") {
		t.Fatalf("audio not cached")
	}
}
