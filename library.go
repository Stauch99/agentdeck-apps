/**
 * [INPUT]: Depends on encoding/json, os, path/filepath, strings, sync, time from stdlib; Track from suno.go
 * [OUTPUT]: Provides Song, Library and its persistence/media methods
 * [POS]: Persistence layer of openmusic; written by Service, read by Server; state lives on the bind-mounted volume
 * [PROTOCOL]: When changing, update this header, then check openmusic/CLAUDE.md
 */
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Song is one library entry. Placeholders use ID=taskID#idx until materialized.
type Song struct {
	ID           string  `json:"id"`
	TaskID       string  `json:"taskId"`
	Idx          int     `json:"idx"`
	Title        string  `json:"title"`
	Style        string  `json:"style"`
	Tags         string  `json:"tags"`
	Lyrics       string  `json:"lyrics,omitempty"` // from kie track.prompt; "[Instrumental]" for instrumental
	Model        string  `json:"model"`
	Duration     float64 `json:"duration"`
	Status       string  `json:"status"` // generating | done | error
	ErrorMessage string  `json:"errorMessage,omitempty"`
	CreatedAt    string  `json:"createdAt"`
	HasAudio     bool    `json:"hasAudio"`
	HasCover     bool    `json:"hasCover"`
}

// Library is the thread-safe JSON-backed song store. Newest songs sort first.
type Library struct {
	mu    sync.Mutex
	dir   string
	path  string
	songs []Song
}

func NewLibrary(dir string) (*Library, error) {
	if err := os.MkdirAll(filepath.Join(dir, "media"), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir media: %w", err)
	}
	l := &Library{dir: dir, path: filepath.Join(dir, "library.json")}
	if b, err := os.ReadFile(l.path); err == nil {
		var wrap struct {
			Songs []Song `json:"songs"`
		}
		if err := json.Unmarshal(b, &wrap); err != nil {
			return nil, fmt.Errorf("parse library.json: %w", err)
		}
		l.songs = wrap.Songs
	}
	return l, nil
}

// List returns a snapshot copy, newest first.
func (l *Library) List() []Song {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Song, len(l.songs))
	copy(out, l.songs)
	return out
}

func (l *Library) AddPlaceholders(taskID, model, style, title string, n int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().Format(time.RFC3339)
	pre := make([]Song, 0, n)
	for i := 0; i < n; i++ {
		pre = append(pre, Song{
			ID: fmt.Sprintf("%s#%d", taskID, i), TaskID: taskID, Idx: i,
			Title: title, Style: style, Model: model,
			Status: "generating", CreatedAt: now,
		})
	}
	l.songs = append(pre, l.songs...) // newest first
	l.save()
}

func (l *Library) Materialize(taskID string, idx int, t Track) (Song, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.songs {
		if l.songs[i].TaskID == taskID && l.songs[i].Idx == idx {
			l.songs[i].ID = t.ID
			if t.Title != "" {
				l.songs[i].Title = t.Title
			}
			l.songs[i].Tags = t.Tags
			l.songs[i].Lyrics = t.Prompt
			if t.Duration > 0 {
				l.songs[i].Duration = t.Duration
			}
			l.save()
			return l.songs[i], nil
		}
	}
	return Song{}, fmt.Errorf("placeholder not found: %s#%d", taskID, idx)
}

func (l *Library) SaveMedia(id, ext string, data []byte) error {
	p := l.MediaPath(id, ext)
	if p == "" {
		return fmt.Errorf("invalid media id: %q", id)
	}
	return os.WriteFile(p, data, 0o644)
}

func (l *Library) MarkDone(id string, hasAudio, hasCover bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.songs {
		if l.songs[i].ID == id {
			l.songs[i].Status = "done"
			l.songs[i].HasAudio = hasAudio
			l.songs[i].HasCover = hasCover
			l.save()
			return nil
		}
	}
	return fmt.Errorf("song not found: %s", id)
}

// SetLyrics fills in a song's lyrics by id (used to backfill songs materialized before lyrics capture existed).
func (l *Library) SetLyrics(id, lyrics string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.songs {
		if l.songs[i].ID == id && l.songs[i].Lyrics != lyrics {
			l.songs[i].Lyrics = lyrics
			l.save()
			return
		}
	}
}

func (l *Library) MarkError(taskID, msg string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	found := false
	for i := range l.songs {
		// Only flip songs that are still generating — never clobber an already-done track.
		if l.songs[i].TaskID == taskID && l.songs[i].Status == "generating" {
			l.songs[i].Status = "error"
			l.songs[i].ErrorMessage = msg
			found = true
		}
	}
	if found {
		l.save()
	}
	return nil
}

func (l *Library) Delete(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := l.songs[:0]
	for _, s := range l.songs {
		if s.ID == id {
			os.Remove(l.MediaPath(id, "mp3"))
			os.Remove(l.MediaPath(id, "jpg"))
			continue
		}
		out = append(out, s)
	}
	l.songs = out
	l.save()
	return nil
}

// MediaPath returns the on-disk path for a media file, or "" if id is unsafe.
func (l *Library) MediaPath(id, ext string) string {
	if id == "" || strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
		return ""
	}
	return filepath.Join(l.dir, "media", id+"."+ext)
}

// save writes library.json. Caller must hold l.mu.
func (l *Library) save() {
	b, _ := json.MarshalIndent(struct {
		Songs []Song `json:"songs"`
	}{l.songs}, "", "  ")
	if err := os.WriteFile(l.path, b, 0o600); err != nil {
		log.Printf("library: save %s failed: %v", l.path, err)
	}
}
