package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type AudioChunk struct {
	ChunkID   string    `json:"chunk_id"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	Timestamp time.Time `json:"timestamp"`
	Data      []byte    `json:"-"`
}

type Metadata struct {
	ChunkID    string    `json:"chunk_id"`
	UserID     string    `json:"user_id"`
	SessionID  string    `json:"session_id"`
	Timestamp  time.Time `json:"timestamp"`
	Checksum   string    `json:"checksum"`
	FFT        string    `json:"fft"`
	Transcript string    `json:"transcript"`
}

type MemoryStore struct {
	mu       sync.RWMutex
	metadata map[string]Metadata
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{metadata: make(map[string]Metadata)}
}

func (s *MemoryStore) Save(meta Metadata) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[meta.ChunkID] = meta
}

func (s *MemoryStore) Get(id string) (Metadata, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.metadata[id]
	return m, ok
}

func (s *MemoryStore) ListByUser(userID string) []Metadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Metadata
	for _, m := range s.metadata {
		if m.UserID == userID {
			result = append(result, m)
		}
	}
	return result
}

type Job struct {
	Chunk  AudioChunk
	Result chan Metadata
}

func TransformStage(ctx context.Context, in <-chan Job) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-in:
			sha := sha256.Sum256(job.Chunk.Data)
			meta := Metadata{
				ChunkID:    job.Chunk.ChunkID,
				UserID:     job.Chunk.UserID,
				SessionID:  job.Chunk.SessionID,
				Timestamp:  job.Chunk.Timestamp,
				Checksum:   fmt.Sprintf("%x", sha),
				FFT:        fmt.Sprintf("%dHz", rand.Intn(10000)),
				Transcript: "Hello World",
			}
			job.Result <- meta
		}
	}
}

var upgrader = websocket.Upgrader{}

func handleUpload(store *MemoryStore, jobs chan Job) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := make([]byte, r.ContentLength)
		r.Body.Read(data)

		userID := r.URL.Query().Get("user_id")
		sessionID := r.URL.Query().Get("session_id")

		chunk := AudioChunk{
			ChunkID:   uuid.New().String(),
			UserID:    userID,
			SessionID: sessionID,
			Timestamp: time.Now(),
			Data:      data,
		}

		result := make(chan Metadata)
		jobs <- Job{Chunk: chunk, Result: result}

		meta := <-result
		store.Save(meta)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(meta)
	}
}

func handleGetChunk(store *MemoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["id"]
		if meta, ok := store.Get(id); ok {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(meta)
		} else {
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	}
}

func handleGetUserSessions(store *MemoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := mux.Vars(r)["user_id"]
		result := store.ListByUser(userID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleWebSocket(store *MemoryStore, jobs chan Job) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, "WebSocket upgrade failed", http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			chunk := AudioChunk{
				ChunkID:   uuid.New().String(),
				UserID:    "user1",
				SessionID: "sess1",
				Timestamp: time.Now(),
				Data:      msg,
			}

			result := make(chan Metadata)
			jobs <- Job{Chunk: chunk, Result: result}

			meta := <-result
			store.Save(meta)
			_ = conn.WriteJSON(map[string]any{
				"ack":        true,
				"chunk_id":   meta.ChunkID,
				"metadata":   meta,
				"transcript": meta.Transcript,
			})
		}
	}
}

// --- Main ---
func main() {
	store := NewMemoryStore()
	jobs := make(chan Job, 100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go TransformStage(ctx, jobs)

	r := mux.NewRouter()
	r.HandleFunc("/upload", handleUpload(store, jobs)).Methods("POST")
	r.HandleFunc("/chunks/{id}", handleGetChunk(store)).Methods("GET")
	r.HandleFunc("/sessions/{user_id}", handleGetUserSessions(store)).Methods("GET")
	r.HandleFunc("/ws", handleWebSocket(store, jobs)).Methods("GET")

	go func() {
		log.Println("Server running on :9090")
		http.ListenAndServe(":9090", r)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down...")
}
