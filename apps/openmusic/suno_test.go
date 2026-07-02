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
	// kie REQUIRES callBackUrl (422 without it) even though we poll — it must be present; customMode/model too.
	if cb, _ := gotBody["callBackUrl"].(string); cb == "" {
		t.Errorf("callBackUrl must be sent (kie requires it), got %v", gotBody["callBackUrl"])
	}
	if gotBody["model"] != "V4_5ALL" || gotBody["customMode"] != true {
		t.Errorf("body missing fields: %v", gotBody)
	}
}

func TestGenerateFoldsStyleVocalIntoPromptInSimpleMode(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		w.Write([]byte(`{"code":200,"msg":"ok","data":{"taskId":"T"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "k")
	// simple mode: kie ignores style/vocalGender, so they must be folded into the prompt to take effect
	c.Generate(context.Background(), GenerateRequest{CustomMode: false, Model: "V4", Prompt: "a song about cats", Style: "romantic ballad", VocalGender: "f"})
	if p, _ := body["prompt"].(string); !strings.Contains(p, "romantic ballad") || !strings.Contains(p, "female vocals") {
		t.Fatalf("simple-mode prompt should fold style+vocal, got %q", p)
	}
	// custom mode: structured fields are honored, so the prompt (=lyrics) is left untouched
	c.Generate(context.Background(), GenerateRequest{CustomMode: true, Model: "V4", Prompt: "my lyrics", Style: "pop", Title: "x", VocalGender: "m"})
	if p, _ := body["prompt"].(string); p != "my lyrics" {
		t.Fatalf("custom-mode prompt should be untouched, got %q", p)
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

// TestRecordInfoParsesRealResponse locks the real kie.ai shape: createTime is a NUMBER (not string),
// and the payload carries extra source* fields. This is the exact shape that broke the first live run.
func TestRecordInfoParsesRealResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":200,"msg":"success","data":{"taskId":"T","status":"SUCCESS","errorCode":null,"errorMessage":null,"response":{"taskId":"T","sunoData":[
			{"id":"uuid-0","audioUrl":"https://tempfile.x/a0.mp3","sourceAudioUrl":"https://cdn1.suno.ai/a0.mp3","streamAudioUrl":"https://musicfile.kie.ai/s0","imageUrl":"https://musicfile.kie.ai/i0.jpeg","prompt":"[Instrumental]","modelName":"chirp-auk-turbo","title":"Midnight Study Loop","tags":"lo-fi","createTime":1782616959537,"duration":149.48}
		]}}}`))
	}))
	defer srv.Close()
	status, tracks, errMsg, err := NewClient(srv.URL, "k").RecordInfo(context.Background(), "T")
	if err != nil {
		t.Fatalf("RecordInfo errored on real shape: %v", err)
	}
	if status != "SUCCESS" || errMsg != "" || len(tracks) != 1 {
		t.Fatalf("RecordInfo => %s %q %d tracks", status, errMsg, len(tracks))
	}
	if tracks[0].ID != "uuid-0" || tracks[0].Title != "Midnight Study Loop" || tracks[0].Duration != 149.48 || tracks[0].CreateTime != 1782616959537 {
		t.Fatalf("track not parsed: %+v", tracks[0])
	}
}

func TestBackfillLyrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":200,"msg":"ok","data":{"status":"SUCCESS","response":{"sunoData":[
			{"id":"old0","audioUrl":"x","prompt":"[Verse]\nbackfilled lyrics","title":"Old","duration":100}
		]}}}`))
	}))
	defer srv.Close()

	lib, _ := NewLibrary(t.TempDir())
	lib.AddPlaceholders("oldtask", "V4", "", "Old", 1)
	lib.Materialize("oldtask", 0, Track{ID: "old0", Title: "Old"}) // no Prompt -> lyrics empty (pre-feature song)
	lib.MarkDone("old0", true, false)
	if lib.List()[0].Lyrics != "" {
		t.Fatalf("precondition: lyrics should start empty")
	}

	NewService(NewClient(srv.URL, "k"), lib).BackfillLyrics(context.Background())

	if got := lib.List()[0].Lyrics; got != "[Verse]\nbackfilled lyrics" {
		t.Fatalf("lyrics not backfilled: %q", got)
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

// TestServicePollRetriesLaggingTrack locks the real-world case that broke live: at FIRST_SUCCESS the
// second track's audioUrl is still empty, so it must be skipped (not errored) and retried at SUCCESS.
func TestServicePollRetriesLaggingTrack(t *testing.T) {
	calls := 0
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/generate") && r.Method == http.MethodPost:
			w.Write([]byte(`{"code":200,"msg":"ok","data":{"taskId":"TL"}}`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/generate/record-info"):
			calls++
			if calls == 1 { // FIRST_SUCCESS: a0 ready, a1 audioUrl still empty
				fmt.Fprintf(w, `{"code":200,"msg":"ok","data":{"status":"FIRST_SUCCESS","response":{"sunoData":[
					{"id":"a0","audioUrl":"%s/m/a0.mp3","imageUrl":"%s/m/a0.jpg","title":"One","duration":120},
					{"id":"a1","audioUrl":"","title":"Two"}
				]}}}`, base, base)
				return
			}
			fmt.Fprintf(w, `{"code":200,"msg":"ok","data":{"status":"SUCCESS","response":{"sunoData":[
				{"id":"a0","audioUrl":"%s/m/a0.mp3","imageUrl":"%s/m/a0.jpg","title":"One","duration":120},
				{"id":"a1","audioUrl":"%s/m/a1.mp3","imageUrl":"%s/m/a1.jpg","title":"Two","duration":121}
			]}}}`, base, base, base, base)
		case strings.HasPrefix(r.URL.Path, "/m/"):
			w.Write([]byte("BINARY:" + r.URL.Path))
		}
	}))
	defer srv.Close()
	base = srv.URL

	lib, _ := NewLibrary(t.TempDir())
	svc := NewService(NewClient(srv.URL, "k"), lib)
	svc.interval = 5 * time.Millisecond
	svc.timeout = 2 * time.Second
	svc.blockPrivateHosts = false

	if _, err := svc.Submit(context.Background(), GenerateRequest{CustomMode: false, Model: "V4", Prompt: "hi"}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
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
		if s.Status != "done" || !s.HasAudio {
			t.Fatalf("lagging track was not retried to done: %+v", s)
		}
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
