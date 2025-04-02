package handlers

import (
	"log"
	"time"
	"vesta/messages"

	"github.com/gorilla/websocket"
)

func handleConnection(ws *websocket.Conn, ticketId string) error {
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	queueTicker := time.NewTicker(250 * time.Millisecond)
	defer queueTicker.Stop()

	done := make(chan struct{})
	defer close(done)

	go func() {
		defer func() { done <- struct{}{} }()
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("read error: %v", err)
				}
				return
			}
		}
	}()

	lastSentCount := getClientCount()

	for {
		select {
		case <-done:
			return nil
		case <-pingTicker.C:
			if err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
				return err
			}
		case <-queueTicker.C:
			currentCount := getClientCount()
			if currentCount != lastSentCount {
				if err := messages.SendQueued(ws, ticketId, currentCount); err != nil {
					return err
				}
				lastSentCount = currentCount
			}
		}
	}
}
