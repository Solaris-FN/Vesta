package utils

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

func SendMessage(ws *websocket.Conn, msg map[string]interface{}) error {
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		log.Printf("JSON marshaling error: %v", err)
		return err
	}

	err = ws.WriteMessage(websocket.TextMessage, jsonMsg)
	if err != nil {
		log.Printf("WebSocket write error: %v", err)
		return err
	}

	return nil
}
