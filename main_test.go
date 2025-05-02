package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestMemoryStore_SaveAndGet(t *testing.T) {
	store := NewMemoryStore()

	meta := Metadata{
		ChunkID:    "chunk1",
		UserID:     "user1",
		SessionID:  "session1",
		Timestamp:  time.Now(),
		Checksum:   "checksum1",
		FFT:        "100Hz",
		Transcript: "Test transcript",
	}

	store.Save(meta)

	retrievedMeta, ok := store.Get("chunk1")
	if !ok {
		t.Errorf("Expected metadata for chunk1, but got none")
	}

	if retrievedMeta.ChunkID != meta.ChunkID {
		t.Errorf("Expected ChunkID %v, but got %v", meta.ChunkID, retrievedMeta.ChunkID)
	}
}

func TestMemoryStore_ListByUser(t *testing.T) {
	store := NewMemoryStore()

	meta1 := Metadata{
		ChunkID:    "chunk1",
		UserID:     "user1",
		SessionID:  "session1",
		Timestamp:  time.Now(),
		Checksum:   "checksum1",
		FFT:        "100Hz",
		Transcript: "Test transcript",
	}

	meta2 := Metadata{
		ChunkID:    "chunk2",
		UserID:     "user1",
		SessionID:  "session2",
		Timestamp:  time.Now(),
		Checksum:   "checksum2",
		FFT:        "200Hz",
		Transcript: "Test transcript 2",
	}

	store.Save(meta1)
	store.Save(meta2)

	metadataList := store.ListByUser("user1")
	if len(metadataList) != 2 {
		t.Errorf("Expected 2 metadata for user1, but got %v", len(metadataList))
	}
}

func TestTransformStage(t *testing.T) {
	in := make(chan Job, 1)
	result := make(chan Metadata)

	chunk := AudioChunk{
		ChunkID:   "chunk1",
		UserID:    "user1",
		SessionID: "session1",
		Timestamp: time.Now(),
		Data:      []byte("audio data"),
	}

	job := Job{Chunk: chunk, Result: result}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go TransformStage(ctx, in)

	in <- job

	meta := <-result

	if meta.ChunkID != chunk.ChunkID {
		t.Errorf("Expected ChunkID %v, but got %v", chunk.ChunkID, meta.ChunkID)
	}

	if meta.Checksum == "" {
		t.Errorf("Expected a checksum, but got an empty value")
	}

	if meta.Transcript == "" {
		t.Errorf("Expected a transcript, but got an empty value")
	}
}

func TestHandleUpload(t *testing.T) {
	filePath := "/home/kundhavk/Downloads/sample_audio.mp3"
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open file %s: %v", filePath, err)
	}
	defer file.Close()

	fileContent, err := ioutil.ReadAll(file)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", filePath, err)
	}

	req, err := http.NewRequest("POST", "/upload?user_id=user1&session_id=sess1", bytes.NewReader(fileContent))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	rr := httptest.NewRecorder()

	store := NewMemoryStore()
	jobs := make(chan Job, 100)

	handler := handleUpload(store, jobs)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code 200, but got %v", rr.Code)
	}

	var response map[string]interface{}
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Errorf("Failed to decode response body: %v", err)
	}

	if _, ok := response["chunk_id"]; !ok {
		t.Errorf("Expected chunk_id in response, but got none")
	}
	if _, ok := response["checksum"]; !ok {
		t.Errorf("Expected checksum in response, but got none")
	}

	checksum := response["checksum"].(string)
	expectedChecksum := getFileChecksum(filePath)
	if checksum != expectedChecksum {
		t.Errorf("Expected checksum %v, but got %v", expectedChecksum, checksum)
	}
}

func getFileChecksum(filePath string) string {
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return ""
	}

	hash := sha256.New()
	hash.Write(fileContent)
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func TestHandleGetChunk(t *testing.T) {
	store := NewMemoryStore()

	meta := Metadata{
		ChunkID:    "chunk1",
		UserID:     "user1",
		SessionID:  "session1",
		Timestamp:  time.Now(),
		Checksum:   "checksum1",
		FFT:        "100Hz",
		Transcript: "Test transcript",
	}
	store.Save(meta)

	req, err := http.NewRequest("GET", "/chunks/14df6751-9bec-41ad-a4d2-70ceb8bef736", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	handler := handleGetChunk(store)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code 200, but got %v", rr.Code)
	}

	var response map[string]interface{}
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Errorf("Failed to decode response body: %v", err)
	}

	if response["chunk_id"] != "chunk1" {
		t.Errorf("Expected chunk_id 'chunk1', but got %v", response["chunk_id"])
	}
}

func TestHandleGetUserSessions(t *testing.T) {
	store := NewMemoryStore()

	meta1 := Metadata{
		ChunkID:    "chunk1",
		UserID:     "user1",
		SessionID:  "session1",
		Timestamp:  time.Now(),
		Checksum:   "checksum1",
		FFT:        "100Hz",
		Transcript: "Test transcript",
	}

	meta2 := Metadata{
		ChunkID:    "chunk2",
		UserID:     "user1",
		SessionID:  "session2",
		Timestamp:  time.Now(),
		Checksum:   "checksum2",
		FFT:        "200Hz",
		Transcript: "Test transcript 2",
	}

	store.Save(meta1)
	store.Save(meta2)

	req, err := http.NewRequest("GET", "/sessions/user1", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	handler := handleGetUserSessions(store)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code 200, but got %v", rr.Code)
	}

	var response []map[string]interface{}
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Errorf("Failed to decode response body: %v", err)
	}

	if len(response) != 2 {
		t.Errorf("Expected 2 sessions for user1, but got %v", len(response))
	}
}

func TestHandleWebSocket(t *testing.T) {
	// This would require mocking WebSocket handling which is more complex and typically involves using a package like `github.com/gorilla/websocket`.
	// You would mock WebSocket client-server interactions in a more complex test setup.

	// This test is left out for brevity. A proper test would mock WebSocket client connections and simulate message exchanges.
}
