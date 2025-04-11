package handlers

import (
	"log"
	"strings"
	"time"
	"vesta/database"
	"vesta/database/entities"
	"vesta/messages"
	"vesta/utils"

	"github.com/gorilla/websocket"
	"github.com/lib/pq"
)

func HandleStates(client Client, ticketId string) error {
	db := database.Get()
	var session entities.Session
	result := db.Where("region = ? AND playlist = ? AND version = ? AND accessible = ?", client.Payload.Region, client.Payload.Playlist, client.Payload.Version, true).First(&session)
	if result.Error != nil {
		if result.Error.Error() != "record not found" {
			log.Printf("Error fetching session: %v", result.Error)
		}
	} else {
		newPlayer := entities.Player{
			AccountID: client.Payload.AccountID,
			Session:   session.Session,
			Team:      pq.StringArray(strings.Split(client.Payload.PartyPlayerIDs, ",")),
		}

		if err := db.Create(&newPlayer).Error; err != nil {
			utils.LogError("Failed to create player: %v", err)
		}

		if err := messages.SendSessionAssignment(client.Conn, session.Session); err != nil {
			utils.LogError("Failed to send session assignment: %v", err)
		}

		if session.Available {
			time.Sleep(500 * time.Millisecond)
			if err := messages.SendJoin(client.Conn, session.Session, session.Session); err != nil {
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
			_, _, err := client.Conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("read error: %v", err)
				}
				return
			}
		}
	}()

	lastSentCount := GetAllClientsViaDataLen(
		client.Payload.Version,
		client.Payload.Playlist,
		client.Payload.Region,
	)

	for {
		select {
		case <-done:
			return nil
		case <-pingTicker.C:
			if err := client.Conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
				return err
			}
		case <-queueTicker.C:
			var updatedSession entities.Session
			updateResult := db.Where("region = ? AND playlist = ? AND version = ? AND accessible = ?", client.Payload.Region, client.Payload.Playlist, client.Payload.Version, true).First(&updatedSession)
			if updateResult.Error == nil {
				currentCount := GetAllClientsViaDataLen(
					client.Payload.Version,
					client.Payload.Playlist,
					client.Payload.Region,
				)
				if currentCount != lastSentCount {
					if err := messages.SendQueued(client.Conn, ticketId, currentCount); err != nil {
						return err
					}
					lastSentCount = currentCount
				}
			}
		}
	}
}
