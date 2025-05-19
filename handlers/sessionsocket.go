package handlers

import (
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

	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	ws.WriteMessage(websocket.TextMessage, []byte(`{"name":"Registered","payload":{}}`))

	done := make(chan struct{})

	defer close(done)

	ticker := time.NewTicker(30 * time.Millisecond)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Printf("Ticker fired. IsAssigned: %v, IsSending: %v, IsAssigning: %v",
					server.IsAssigned, server.IsSending, server.IsAssigning)

				if !server.IsSending && !server.IsAssigning {
					SelectPlaylist(server.SessionId, server.Payload.Region)
					log.Printf("Session - %s has selected a playlist", server.SessionId)
				} else {
					log.Printf("Conditions not met, stopping ticker")
					return
				}
			case <-done:
				log.Printf("Connection closed, stopping ticker")
				return
			}
		}
	}()

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			utils.LogError("failed to read message: %v", err)
			break
		}
		log.Printf("Received message: %s", message)
		var data map[string]interface{}
		if err := json.Unmarshal(message, &data); err != nil {
			if string(message) != "ping" {
				utils.LogError("failed to decode message: %v", err)
				close(done)
			}
			return
		}

		if name, ok := data["name"].(string); ok && name == "AssignMatchResult" {
			payload, ok := data["payload"].(map[string]interface{})
			if !ok {
				return
			}
			if result, ok := payload["result"].(string); ok {
				if result == "failed" {
				} else if result == "ready" {
					utils.LogInfo("Session - %s has AssignedMatch", server.SessionId)

					time.Sleep(2 * time.Second)
					server.IsAssigned = true
					for _, client := range GetAllClientsViaData(
						server.Payload.Version,
						server.Playlist,
						server.Payload.Region,
					) {
						if err := messages.SendJoin(client.Conn, server.SessionId, server.SessionId); err != nil {
							utils.LogError("Failed to send join: %v", err)
						}
					}
					time.Sleep(3 * time.Second)
					server.StopAllowingConnections = true
				}
			}
		}
	}
}
