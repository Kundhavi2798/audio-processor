package main

import (
	"fmt"
	"log"

	"github.com/gorilla/websocket"
)

func main() {
	url := "ws://localhost:9090/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("Dial error:", err)
	}
	defer conn.Close()

	// Send message
	err = conn.WriteMessage(websocket.TextMessage, []byte("Hello from Go client"))
	if err != nil {
		log.Fatal("Write error:", err)
	}

	// Read reply
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Fatal("Read error:", err)
	}
	fmt.Println("Received from server:", string(msg))
}
