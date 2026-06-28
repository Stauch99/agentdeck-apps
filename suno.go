/**
 * [INPUT]: Depends on net/http, net, encoding/json, context, io, net/url, strings, time from stdlib; Library from this module
 * [OUTPUT]: Provides Track, GenerateRequest, Client, Service for kie.ai SunoAPI
 * [POS]: kie.ai-facing layer of openmusic; consumed by Server, writes via Library
 * [PROTOCOL]: When changing, update this header, then check openmusic/CLAUDE.md
 */
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Track is one generated candidate as returned by kie.ai record-info sunoData[].
type Track struct {
	ID             string  `json:"id"`
	AudioURL       string  `json:"audioUrl"`
	StreamAudioURL string  `json:"streamAudioUrl"`
	ImageURL       string  `json:"imageUrl"`
	Title          string  `json:"title"`
	Tags           string  `json:"tags"`
	Prompt         string  `json:"prompt"`
	ModelName      string  `json:"modelName"`
	Duration       float64 `json:"duration"`
	CreateTime     string  `json:"createTime"`
}

var validModels = map[string]bool{
	"V3_5": true, "V4": true, "V4_5": true, "V4_5PLUS": true,
	"V4_5ALL": true, "V5": true, "V5_5": true,
}

// GenerateRequest is the validated, internal generation request.
type GenerateRequest struct {
	CustomMode          bool     `json:"customMode"`
	Model               string   `json:"model"`
	Instrumental        bool     `json:"instrumental"`
	Prompt              string   `json:"prompt"`
	Style               string   `json:"style"`
	NegativeTags        string   `json:"negativeTags"`
	Title               string   `json:"title"`
	VocalGender         string   `json:"vocalGender"`         // "", "m", "f"
	StyleWeight         *float64 `json:"styleWeight"`         // 0..1
	WeirdnessConstraint *float64 `json:"weirdnessConstraint"` // 0..1
}

func (r GenerateRequest) Validate() error {
	if !validModels[r.Model] {
		return fmt.Errorf("invalid model %q", r.Model)
	}
	if r.CustomMode {
		if r.Style == "" || r.Title == "" {
			return fmt.Errorf("custom mode requires style and title")
		}
		if !r.Instrumental && r.Prompt == "" {
			return fmt.Errorf("custom non-instrumental requires lyrics prompt")
		}
		return nil
	}
	if r.Prompt == "" {
		return fmt.Errorf("simple mode requires prompt")
	}
	return nil
}

// envelope is the kie.ai top-level response wrapper.
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// kieGenBody is the wire body; omitempty drops unset optionals and never adds callBackUrl.
type kieGenBody struct {
	Prompt              string   `json:"prompt,omitempty"`
	CustomMode          bool     `json:"customMode"`
	Instrumental        bool     `json:"instrumental"`
	Model               string   `json:"model"`
	Style               string   `json:"style,omitempty"`
	Title               string   `json:"title,omitempty"`
	NegativeTags        string   `json:"negativeTags,omitempty"`
	VocalGender         string   `json:"vocalGender,omitempty"`
	StyleWeight         *float64 `json:"styleWeight,omitempty"`
	WeirdnessConstraint *float64 `json:"weirdnessConstraint,omitempty"`
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*envelope, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = strings.NewReader(string(b))
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call kie %s: %w", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parse kie response (%d): %w", resp.StatusCode, err)
	}
	if env.Code != 200 {
		return nil, fmt.Errorf("kie %s failed: code %d: %s", path, env.Code, env.Msg)
	}
	return &env, nil
}

func (c *Client) Generate(ctx context.Context, r GenerateRequest) (string, error) {
	env, err := c.do(ctx, http.MethodPost, "/api/v1/generate", kieGenBody{
		Prompt: r.Prompt, CustomMode: r.CustomMode, Instrumental: r.Instrumental,
		Model: r.Model, Style: r.Style, Title: r.Title, NegativeTags: r.NegativeTags,
		VocalGender: r.VocalGender, StyleWeight: r.StyleWeight, WeirdnessConstraint: r.WeirdnessConstraint,
	})
	if err != nil {
		return "", err
	}
	var d struct {
		TaskID string `json:"taskId"`
	}
	if err := json.Unmarshal(env.Data, &d); err != nil {
		return "", fmt.Errorf("parse kie generate data: %w", err)
	}
	if d.TaskID == "" {
		return "", fmt.Errorf("kie returned empty taskId")
	}
	return d.TaskID, nil
}

// RecordInfo returns (status, tracks, errMsg, err). errMsg carries kie's errorMessage on failure states.
func (c *Client) RecordInfo(ctx context.Context, taskID string) (string, []Track, string, error) {
	env, err := c.do(ctx, http.MethodGet, "/api/v1/generate/record-info?taskId="+url.QueryEscape(taskID), nil)
	if err != nil {
		return "", nil, "", err
	}
	var d struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"errorMessage"`
		Response     struct {
			SunoData []Track `json:"sunoData"`
		} `json:"response"`
	}
	if err := json.Unmarshal(env.Data, &d); err != nil {
		return "", nil, "", fmt.Errorf("parse kie record-info data: %w", err)
	}
	return d.Status, d.Response.SunoData, d.ErrorMessage, nil
}

// Service ties the kie client to the library: submit + background poll + media cache.
type Service struct {
	client            *Client
	lib               *Library
	http              *http.Client
	interval          time.Duration
	timeout           time.Duration
	baseCtx           context.Context // process-lifetime context; cancelled on shutdown to stop poll loops
	blockPrivateHosts bool            // SSRF guard: refuse media downloads from loopback/private hosts
	maxFetchBytes     int64           // cap per media download (anti memory-exhaustion)
}

func NewService(c *Client, lib *Library) *Service {
	return &Service{
		client:            c,
		lib:               lib,
		http:              &http.Client{Timeout: 60 * time.Second},
		interval:          10 * time.Second,
		timeout:           10 * time.Minute,
		baseCtx:           context.Background(),
		blockPrivateHosts: true,
		maxFetchBytes:     64 << 20, // 64 MiB
	}
}

// Submit validates, calls kie generate, seeds two placeholders, and spawns the poll loop.
// The poll runs under baseCtx (process lifetime), NOT the request ctx, so it survives the HTTP response.
func (s *Service) Submit(ctx context.Context, r GenerateRequest) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	taskID, err := s.client.Generate(ctx, r)
	if err != nil {
		return "", err
	}
	s.lib.AddPlaceholders(taskID, r.Model, r.Style, r.Title, 2)
	go s.poll(s.baseCtx, taskID)
	return taskID, nil
}

// poll drives one task to completion: 10s ticks until SUCCESS/failure/timeout, or until ctx is cancelled
// (graceful shutdown). Tracks are deduplicated by track ID and assigned stable placeholder slots in arrival
// order, so a reorder between FIRST_SUCCESS and SUCCESS never skips or double-writes a candidate.
func (s *Service) poll(ctx context.Context, taskID string) {
	slot := map[string]int{} // trackID -> placeholder index (stable, arrival order)
	next := 0
	deadline := time.Now().Add(s.timeout)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return // process shutting down
		case <-ticker.C:
		}
		if time.Now().After(deadline) {
			s.lib.MarkError(taskID, "generation timed out")
			return
		}
		rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		status, tracks, errMsg, err := s.client.RecordInfo(rctx, taskID)
		cancel()
		if err != nil {
			continue // transient; retry next tick
		}
		switch status {
		case "PENDING", "TEXT_SUCCESS":
			// keep waiting
		case "FIRST_SUCCESS", "SUCCESS":
			for _, tr := range tracks {
				if _, seen := slot[tr.ID]; seen {
					continue // already handled this track in an earlier poll
				}
				if next >= 2 {
					continue // only two candidate slots exist per task
				}
				idx := next
				next++
				slot[tr.ID] = idx
				if _, err := s.lib.Materialize(taskID, idx, tr); err != nil {
					continue
				}
				s.cacheMedia(tr)
			}
			if status == "SUCCESS" {
				return
			}
		default: // CREATE_TASK_FAILED / GENERATE_AUDIO_FAILED / CALLBACK_EXCEPTION / SENSITIVE_WORD_ERROR
			msg := errMsg
			if msg == "" {
				msg = status
			}
			s.lib.MarkError(taskID, msg)
			return
		}
	}
}

// cacheMedia downloads audio+cover into the volume and flips the song to done.
func (s *Service) cacheMedia(t Track) {
	hasAudio := false
	if b, err := s.fetch(t.AudioURL); err == nil {
		if s.lib.SaveMedia(t.ID, "mp3", b) == nil {
			hasAudio = true
		}
	}
	hasCover := false
	if t.ImageURL != "" {
		if b, err := s.fetch(t.ImageURL); err == nil {
			if s.lib.SaveMedia(t.ID, "jpg", b) == nil {
				hasCover = true
			}
		}
	}
	if hasAudio {
		s.lib.MarkDone(t.ID, hasAudio, hasCover)
		return
	}
	// audio failed: flip just this song to error by id
	s.lib.markErrorByID(t.ID, "audio download failed")
}

// fetch downloads a media URL with SSRF + size guards: only http/https, optionally refuse private/loopback
// hosts, and cap the body at maxFetchBytes.
func (s *Service) fetch(rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse media url: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("refuse media url scheme %q", u.Scheme)
	}
	if s.blockPrivateHosts && isPrivateHost(u.Hostname()) {
		return nil, fmt.Errorf("refuse private/loopback media host %q", u.Hostname())
	}
	resp, err := s.http.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download %s: http %d", rawURL, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, s.maxFetchBytes))
}

// isPrivateHost reports whether host resolves to a loopback/private/link-local address (SSRF guard).
// Unresolvable or empty hosts are treated as unsafe.
func isPrivateHost(host string) bool {
	if host == "" {
		return true
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return true
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return true
		}
	}
	return false
}
