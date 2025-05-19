package handlers

import (
	"log"
	"strings"
	"time"
	"vesta/classes"
	"vesta/database"
	"vesta/database/entities"
	"vesta/messages"
	"vesta/utils"

	"github.com/gorilla/websocket"
	"github.com/lib/pq"
)

func HandleStates(client Client, ticketId string) error {
	db := database.Get()
	if classes.Config["VestaSessions"] == true {
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
	} else if classes.Config["FortniteSessions"] == true {
		var session entities.MMSessions
		result := db.Where("region = ? AND playlist_name = ?", client.Payload.Region, client.Payload.Playlist).First(&session)
		if result.Error != nil {
			if result.Error.Error() != "record not found" {
				log.Printf("Error fetching session: %v", result.Error)
			}
		} else {
			if err := messages.SendSessionAssignment(client.Conn, session.SessionId); err != nil {
				utils.LogError("Failed to send session assignment: %v", err)
			}

			time.Sleep(500 * time.Millisecond)
			if err := messages.SendJoin(client.Conn, session.SessionId, session.SessionId); err != nil {
				utils.LogError("Failed to send join: %v", err)
			}
		}
	}

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	queueTicker := time.NewTicker(1 * time.Second)
	defer queueTicker.Stop()

	done := make(chan struct{})
	go func() {
		defer func() {
			select {
			case done <- struct{}{}:
			default:
			}
		}()
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
			if classes.Config["VestaSessions"] == true {
				var updatedSession entities.Session
				updateResult := db.Where("region = ? AND playlist = ? AND version = ? AND accessible = ?", client.Payload.Region, client.Payload.Playlist, client.Payload.Version, true).First(&updatedSession)
				if updateResult.Error != nil {
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

			if classes.Config["FortniteSessions"] == true {
				var updatedSession entities.MMSessions
				updateResult := db.Where("region = ? AND playlist_name = ?", client.Payload.Region, client.Payload.Playlist).First(&updatedSession)
				if updateResult.Error != nil {
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
}
