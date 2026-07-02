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
	got, err := lib.Materialize("task1", 0, Track{ID: "aud0", Title: "Real Title", Tags: "lofi,chill", Prompt: "[Verse]\nsunlight on the floor", Duration: 123})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	// the user named it "My Song" (set in AddPlaceholders) — kie's generated title must NOT clobber it
	if got.ID != "aud0" || got.Title != "My Song" || got.Tags != "lofi,chill" || got.Duration != 123 {
		t.Fatalf("materialize did not map track / preserve title: %+v", got)
	}
	if got.Lyrics != "[Verse]\nsunlight on the floor" {
		t.Fatalf("lyrics not captured from track.prompt: %q", got.Lyrics)
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

func TestMaterializeTitlePreservesUserName(t *testing.T) {
	lib, _ := NewLibrary(t.TempDir())
	// user named it -> preserved even if kie returns a different title
	lib.AddPlaceholders("t1", "V4", "", "我的歌名", 1)
	if g, _ := lib.Materialize("t1", 0, Track{ID: "a1", Title: "kie generated"}); g.Title != "我的歌名" {
		t.Fatalf("user title not preserved: %q", g.Title)
	}
	// user left it blank -> borrow kie's generated title
	lib.AddPlaceholders("t2", "V4", "", "", 1)
	if g, _ := lib.Materialize("t2", 0, Track{ID: "a2", Title: "kie generated"}); g.Title != "kie generated" {
		t.Fatalf("blank title should take kie's: %q", g.Title)
	}
}

func TestMediaPathRejectsTraversal(t *testing.T) {
	lib, _ := NewLibrary(t.TempDir())
	if p := lib.MediaPath("../../etc/passwd", "mp3"); p != "" {
		t.Fatalf("traversal not rejected: %s", p)
	}
}
