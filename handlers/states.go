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
	}

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	queueTicker := time.NewTicker(500 * time.Millisecond)
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
			for _, server := range Sessions {
				if server != nil {
					if server.Payload.Region == client.Payload.Region {
						if !server.IsSending && !server.IsAssigning {
							SelectPlaylist(server.SessionId, server.Payload.Region)
						} else if server.IsSending && server.IsAssigning {
							queueTicker.Stop()
							// sesh := Sessions[server.SessionId]
							// if sesh == nil {
							// 	log.Printf("session not found in memory: %+v", server.SessionId)
							// 	continue
							// }

							// if sesh.Teams == nil {
							// 	sesh.Teams = make([][][]string, 0)
							// }

							// ids := strings.Split(client.Payload.PartyPlayerIDs, ",")

							// teamIndex := -1
							// for i, team := range sesh.Teams {
							// 	for _, playerEntry := range team {
							// 		for _, existingId := range playerEntry {
							// 			for _, newId := range ids {
							// 				if existingId == newId {
							// 					teamIndex = i
							// 					break
							// 				}
							// 			}
							// 			if teamIndex != -1 {
							// 				break
							// 			}
							// 		}
							// 		if teamIndex != -1 {
							// 			break
							// 		}
							// 	}
							// 	if teamIndex != -1 {
							// 		break
							// 	}
							// }

							// if teamIndex == -1 {
							// 	teamIndex = len(sesh.Teams)
							// 	sesh.Teams = append(sesh.Teams, make([][]string, 0))
							// }

							// for _, id := range ids {
							// 	exists := false
							// 	for _, playerEntry := range sesh.Teams[teamIndex] {
							// 		for _, existingId := range playerEntry {
							// 			if existingId == id {
							// 				exists = true
							// 				break
							// 			}
							// 		}
							// 		if exists {
							// 			break
							// 		}
							// 	}

							// 	if !exists {
							// 		playerEntry := []string{id}
							// 		sesh.Teams[teamIndex] = append(sesh.Teams[teamIndex], playerEntry)
							// 	}
							// }

							// Sessions[server.SessionId] = sesh

							// payloadBackfill := classes.BackfillMatchPayload{
							// 	Name: "BackfillMatch",
							// 	Payload: classes.BackfillMatchPayloadData{
							// 		Teams:           sesh.Teams,
							// 		BackfillId:      uuid.New().String(),
							// 		BackfillOptions: make(map[string]interface{}),
							// 	},
							// }

							// msgBackfill, err := json.Marshal(payloadBackfill)
							// if err != nil {
							// 	log.Printf("failed to marshal BackfillMatch payload: %v", err)
							// }
							// server.Conn.WriteMessage(websocket.TextMessage, msgBackfill)
						}
					}
				}
			}

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
			} else {
				var updatedSession entities.MMSessions
				updateResult := db.Where("region = ? AND playlist = ? AND started = ?", client.Payload.Region, client.Payload.Playlist, false).First(&updatedSession)

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
