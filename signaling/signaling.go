package ws

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

// Handles incoming WebSocket signaling
func WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}
		fmt.Printf("WebSocket message: %s\n", msg)

		// Attempts to parse a JSON-encoded byte slice `msg` into a generic map[string]interface{}.
		// If the JSON is invalid or cannot be unmarshalled
		//(converting data from a serialized format (like JSON or XML) into a native data structure),
		//logs an error message with details.
		// On success, `payload` will contain the decoded JSON object for further processing.
		var payload map[string]interface{}
		if err := json.Unmarshal(msg, &payload); err != nil {
			log.Println("Invalid JSON:", err)
		}

		// sdpChannel <- msg
		// Handle signaling messages here
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
