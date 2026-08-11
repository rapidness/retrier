package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync/atomic"
)

// Mock LLM API server for demo
// - /normal → always 200 success
// - /code700 → returns code=700 on first 2 calls, then success
// - /rate429 → returns 429 on first 2 calls, then success
// - /bad400 → always returns 400

var code700Count atomic.Int32
var rate429Count atomic.Int32

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/normal", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0, "msg": "success", "data": "hello from LLM API",
		})
	})

	mux.HandleFunc("/code700", func(w http.ResponseWriter, r *http.Request) {
		count := code700Count.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if count <= 2 {
			log.Printf("[upstream] /code700 call #%d → code=700 (quota exceeded)", count)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 700, "msg": "quota exceeded",
			})
		} else {
			log.Printf("[upstream] /code700 call #%d → success", count)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0, "msg": "success", "data": "recovered!",
			})
		}
	})

	mux.HandleFunc("/rate429", func(w http.ResponseWriter, r *http.Request) {
		count := rate429Count.Add(1)
		if count <= 2 {
			log.Printf("[upstream] /rate429 call #%d → 429 Too Many Requests", count)
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(429)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "rate limited", "retry_after": 1,
			})
		} else {
			log.Printf("[upstream] /rate429 call #%d → 200 OK", count)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 0, "msg": "success",
			})
		}
	})

	mux.HandleFunc("/bad400", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[upstream] /bad400 → 400 Bad Request (never retry)")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "invalid request",
		})
	})

	log.Println("Mock upstream listening on :15721")
	log.Fatal(http.ListenAndServe(":15721", mux))
}
