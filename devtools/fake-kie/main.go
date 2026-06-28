// fake-kie: a throwaway local stand-in for api.kie.ai used ONLY for UI verification.
// NOT embedded, NOT in the Docker image. Run: go run ./devtools/fake-kie  (listens :8077)
// Point OpenMusic at it: KIE_BASE_URL=http://localhost:8077
package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

func main() {
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
			{"id":"fake0","audioUrl":"%s/sample.mp3","imageUrl":"%s/sample.jpg","title":"Fake One","tags":"demo","duration":42},
			{"id":"fake1","audioUrl":"%s/sample.mp3","imageUrl":"%s/sample.jpg","title":"Fake Two","tags":"demo","duration":42}
		]}}}`, base, base, base, base)
	})
	// tiny silent-ish wav + 1px jpg so the player and cover have real bytes
	h.HandleFunc("/sample.mp3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write(make([]byte, 4096))
	})
	h.HandleFunc("/sample.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte{0xFF, 0xD8, 0xFF, 0xD9})
	})
	http.ListenAndServe(":8077", h)
}
