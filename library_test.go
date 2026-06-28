package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLibraryLifecycle(t *testing.T) {
	dir := t.TempDir()
	lib, err := NewLibrary(dir)
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}

	// AddPlaceholders creates N "generating" songs keyed taskID#idx.
	lib.AddPlaceholders("task1", "V4_5ALL", "lofi", "My Song", 2)
	songs := lib.List()
	if len(songs) != 2 {
		t.Fatalf("want 2 placeholders, got %d", len(songs))
	}
	if songs[0].Status != "generating" || songs[0].ID != "task1#0" || songs[0].Style != "lofi" {
		t.Fatalf("bad placeholder: %+v", songs[0])
	}

	// Materialize replaces the placeholder ID with the real audioId.
	got, err := lib.Materialize("task1", 0, Track{ID: "aud0", Title: "Real Title", Tags: "lofi,chill", Duration: 123})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if got.ID != "aud0" || got.Title != "Real Title" || got.Tags != "lofi,chill" || got.Duration != 123 {
		t.Fatalf("materialize did not map track: %+v", got)
	}
	if got.Status != "generating" {
		t.Fatalf("status should stay generating until media saved, got %s", got.Status)
	}

	// SaveMedia writes bytes into <dir>/media/<id>.<ext>.
	if err := lib.SaveMedia("aud0", "mp3", []byte("ID3audio")); err != nil {
		t.Fatalf("SaveMedia: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "media", "aud0.mp3")); string(b) != "ID3audio" {
		t.Fatalf("media not written")
	}

	// MarkDone flips status + media flags.
	if err := lib.MarkDone("aud0", true, false); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}

	// Reload from disk → persistence round-trips.
	lib2, err := NewLibrary(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	s := lib2.List()
	if len(s) != 2 || s[0].Status != "done" || !s[0].HasAudio || s[0].HasCover {
		t.Fatalf("persistence wrong: %+v", s[0])
	}

	// MarkError flips only still-generating songs; the already-done track (aud0) is preserved.
	lib2.MarkError("task1", "sensitive words")
	for _, sg := range lib2.List() {
		switch sg.ID {
		case "aud0":
			if sg.Status != "done" {
				t.Fatalf("MarkError clobbered the done song: %+v", sg)
			}
		case "task1#1":
			if sg.Status != "error" || sg.ErrorMessage != "sensitive words" {
				t.Fatalf("MarkError did not flip the generating song: %+v", sg)
			}
		}
	}

	// Delete removes the song and its media file.
	lib2.Delete("aud0")
	if len(lib2.List()) != 1 {
		t.Fatalf("Delete did not drop song")
	}
	if _, err := os.Stat(filepath.Join(dir, "media", "aud0.mp3")); !os.IsNotExist(err) {
		t.Fatalf("Delete left media file")
	}
}

func TestMediaPathRejectsTraversal(t *testing.T) {
	lib, _ := NewLibrary(t.TempDir())
	if p := lib.MediaPath("../../etc/passwd", "mp3"); p != "" {
		t.Fatalf("traversal not rejected: %s", p)
	}
}
