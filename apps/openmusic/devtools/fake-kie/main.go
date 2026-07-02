// fake-kie: a throwaway local stand-in for api.kie.ai used ONLY for UI verification.
// NOT embedded, NOT in the Docker image. Run: go run ./devtools/fake-kie  (listens :8077)
// Point OpenMusic at it: KIE_BASE_URL=http://localhost:8077 OPENMUSIC_ALLOW_PRIVATE_MEDIA=1
// Serves a REAL jpeg cover + real multi-line lyrics so cover sizing + the lyrics modal are testable.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"net/http"
	"sync"
	"time"
)

// sampleJPEG returns a real 256x256 warm-gradient jpeg so covers render at natural size
// (a 4-byte stub hid the cover-overflow bug; a real image reproduces + verifies the fix).
func sampleJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			img.Set(x, y, color.RGBA{R: uint8(225 - y/4), G: uint8(110 + x/4), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	return buf.Bytes()
}

const fakeLyrics = `[Verse]
Neon city nights, chasing dreams on empty streets
Headlights paint the rain, my heart still skips a beat

[Chorus]
We run through the glow, where the wild rivers flow
Hold on, don't let go — this is all that we know`

func main() {
	jpg := sampleJPEG()
	ly, _ := json.Marshal(fakeLyrics) // safe JSON string (handles newlines/quotes)
	var mu sync.Mutex
	started := map[string]time.Time{}
	h := http.NewServeMux()
	h.HandleFunc("/api/v1/generate", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		started["T-fake"] = time.Now()
		mu.Unlock()
		w.Write([]byte(`{"code":200,"msg":"ok","data":{"taskId":"T-fake"}}`))
	})
	h.HandleFunc("/api/v1/generate/record-info", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		age := time.Since(started[r.URL.Query().Get("taskId")])
		mu.Unlock()
		if age < 6*time.Second {
			w.Write([]byte(`{"code":200,"msg":"ok","data":{"status":"PENDING","response":{"sunoData":[]}}}`))
			return
		}
		base := "http://" + r.Host
		fmt.Fprintf(w, `{"code":200,"msg":"ok","data":{"status":"SUCCESS","response":{"sunoData":[
			{"id":"fake0","audioUrl":"%s/sample.mp3","imageUrl":"%s/sample.jpg","title":"Neon Dreams","tags":"synthwave, retro 80s","prompt":%s,"createTime":1782616959537,"duration":42},
			{"id":"fake1","audioUrl":"%s/sample.mp3","imageUrl":"%s/sample.jpg","title":"Neon Dreams","tags":"synthwave, retro 80s","prompt":%s,"createTime":1782616959537,"duration":42}
		]}}}`, base, base, string(ly), base, base, string(ly))
	})
	h.HandleFunc("/sample.mp3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write(make([]byte, 4096))
	})
	h.HandleFunc("/sample.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpg)
	})
	log.Fatal(http.ListenAndServe(":8077", h))
}
