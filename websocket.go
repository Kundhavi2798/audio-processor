package main

import (
	"fmt"
	"net/http"
)

//var upgrader = websocket.Upgrader{
//	CheckOrigin: func(r *http.Request) bool {
//		return true
//	},
//}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Read error:", err)
			break
		}
		fmt.Println("Received:", string(msg))
		conn.WriteMessage(msgType, msg)
	}
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	fmt.Println("Server started at :8081")
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		fmt.Println("Server error:", err)
	}
}
