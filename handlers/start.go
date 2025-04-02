package handlers

import (
	"time"
	"vesta/messages"

	"github.com/gorilla/websocket"
)

func sendInitMessages(ws *websocket.Conn, ticketID string, count int) error {
	if err := messages.SendConnecting(ws); err != nil {
		return err
	}
	time.Sleep(400 * time.Millisecond)

	if err := messages.SendWaiting(ws); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)

	return messages.SendQueued(ws, ticketID, count)
}
