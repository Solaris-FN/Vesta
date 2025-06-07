package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
	"vesta/classes"
	"vesta/messages"
	"vesta/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	Sessions = make(map[string]*classes.Server)
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

func HandleSessionWebSocket(c *gin.Context) {
	w, r := c.Writer, c.Request

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		utils.LogError("failed to upgrade connection: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		utils.LogError("Authorization header is missing")
		c.AbortWithStatus(http.StatusUnauthorized)
		ws.Close()
		return
	}

	authParts := strings.SplitN(authHeader, " ", 4)
	if len(authParts) != 4 || authParts[0] != "Epic-Signed" || authParts[1] != "Vesta-Sessions" {
		utils.LogError("Invalid Authorization header format")
		c.AbortWithStatus(http.StatusUnauthorized)
		ws.Close()
		return
	}

	const JWT_SECRET = "vmkt7lob4n0purvn7n96c3tk8vb5o2a4hu1a8fqisa1xx718bx808ns5si1jhm98qlycpzk8us0b57j8gt5td1c42c1us9ww"

	payload, err := utils.VerifyJWT(authParts[3], JWT_SECRET)
	if err != nil {
		utils.LogError("failed to verify JWT: %v", err)
		c.AbortWithStatus(http.StatusUnauthorized)
		ws.Close()
		return
	}

	server := &classes.Server{
		Conn: ws,
	}

	if bucketID, ok := payload["bucketId"].(string); ok {
		server.Payload.BucketID = bucketID
	}
	if region, ok := payload["region"].(string); ok {
		server.Payload.Region = region
	}
	if version, ok := payload["version"].(string); ok {
		server.Payload.Version = version
	}
	if buildUniqueID, ok := payload["buildUniqueId"].(string); ok {
		server.Payload.BuildUniqueID = buildUniqueID
	}
	if exp, ok := payload["exp"].(float64); ok {
		server.Payload.Exp = int64(exp)
	}
	if iat, ok := payload["iat"].(float64); ok {
		server.Payload.Iat = int64(iat)
	}
	if jti, ok := payload["jti"].(string); ok {
		server.Payload.Jti = jti
	}

	server.MatchId = uuid.New().String()
	server.IsAssigned = false
	server.IsAssigning = false
	server.StopAllowingConnections = false
	server.Playlist = ""
	server.IsSending = false
	server.AssignMatchSent = false
	server.MinPlayers = 2
	server.MaxPlayers = 0
	server.SessionId = authParts[2]
	Sessions[server.SessionId] = server

	ws.SetReadLimit(maxMessageSize)
	ws.SetPongHandler(func(string) error {
		return nil
	})

	ws.WriteMessage(websocket.TextMessage, []byte(`{"name":"Registered","payload":{}}`))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
					utils.LogError("failed to send ping to session %s: %v", server.SessionId, err)
					cancel()
					return
				}
			case <-ctx.Done():
				utils.LogInfo("Ping goroutine stopping for session: %s", server.SessionId)
				return
			}
		}
	}()

	go func() {
		defer func() {
			utils.LogInfo("Message processing goroutine ending for session: %s", server.SessionId)
			cancel()
		}()

		messageCount := 0
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					utils.LogError("websocket error for session %s: %v", server.SessionId, err)
				} else {
					utils.LogInfo("websocket connection closed for session %s: %v", server.SessionId, err)
				}
				return
			}

			select {
			case <-ctx.Done():
				utils.LogInfo("Context cancelled, stopping message processing for session: %s", server.SessionId)
				return
			default:
			}

			messageCount++
			log.Printf("Message #%d received from session %s: %s", messageCount, server.SessionId, message)

			if string(message) == "ping" {
				ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteMessage(websocket.TextMessage, []byte("pong")); err != nil {
					utils.LogError("failed to send pong to session %s: %v", server.SessionId, err)
					return
				}
				continue
			}

			var data map[string]interface{}
			if err := json.Unmarshal(message, &data); err != nil {
				utils.LogError("failed to decode message from session %s: %v", server.SessionId, err)
				continue
			}

			if messageName, ok := data["name"].(string); ok {
				log.Printf("Processing message type '%s' for session %s", messageName, server.SessionId)

				if messageName == "AssignMatchResult" {
					payload, ok := data["payload"].(map[string]interface{})
					if !ok {
						utils.LogError("Invalid payload for AssignMatchResult from session: %s", server.SessionId)
						continue
					}
					if result, ok := payload["result"].(string); ok {
						if result == "failed" {
							utils.LogInfo("Match assignment failed for session %s", server.SessionId)
						} else if result == "ready" {
							utils.LogInfo("Session - %s has AssignedMatch", server.SessionId)

							go func() {
								time.Sleep(2 * time.Second)
								server.IsAssigned = true

								clients := GetAllClientsViaData(
									server.Payload.Version,
									server.Playlist,
									server.Payload.Region,
								)

								if clients != nil {
									for i, client := range clients {
										utils.LogInfo("Processing client %d for session %s", i, server.SessionId)
										if client != nil && client.Conn != nil {
											time.Sleep(100 * time.Millisecond)
											if err := messages.SendJoin(client.Conn, server.SessionId, server.SessionId); err != nil {
												utils.LogError("Failed to send join: %v", err)
											}
										} else {
											utils.LogInfo("Client %d is nil or has nil connection", i)
										}
									}
								} else {
									utils.LogError("No clients found for session %s, playlist %s, region %s, version %s",
										server.SessionId, server.Playlist, server.Payload.Region, server.Payload.Version)
								}

								time.Sleep(3 * time.Second)
								server.StopAllowingConnections = true
							}()
						}
					}
				}
			} else {
				utils.LogError("Message missing 'name' field from session %s", server.SessionId)
			}
		}
	}()

	defer func() {
		utils.LogInfo("Cleaning up session: %s", server.SessionId)
		delete(Sessions, server.SessionId)
		ws.Close()
	}()

	<-ctx.Done()
	utils.LogInfo("Context cancelled, cleaning up session: %s", server.SessionId)
}
