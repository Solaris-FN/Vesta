package handlers

import (
	"log"
	"time"
	"vesta/database"
	"vesta/database/entities"
	"vesta/messages"
	"vesta/utils"

	"github.com/gorilla/websocket"
)

func handleConnection(ws *websocket.Conn, ticketId string, client Client) error {
	db := database.Get()
	var session entities.Session
	result := db.Where("region = ? AND playlist = ? AND version = ?", client.Payload.Region, client.Payload.Playlist, client.Payload.Version).First(&session)
	if result.Error != nil {
		if result.Error.Error() == "record not found" {
			log.Printf("Session not found for region: %s, playlist: %s, version: %s", client.Payload.Region, client.Payload.Playlist, client.Payload.Version)
		}
	} else {
		if err := messages.SendSessionAssignment(client.Conn, session.Session); err != nil {
			utils.LogError("Failed to send session assignment: %v", err)
		}

		if session.Available {
			time.Sleep(500 * time.Millisecond)
			if err := messages.SendJoin(ws, session.Session, session.Session); err != nil {
				utils.LogError("Failed to send join: %v", err)
			}
		}
	}

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
