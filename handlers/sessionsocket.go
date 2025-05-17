package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"vesta/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Server struct {
	Conn    *websocket.Conn
	Payload struct {
		BucketID      interface{} `json:"bucketId"`
		Region        string      `json:"region"`
		Version       string      `json:"version"`
		BuildUniqueID string      `json:"buildUniqueId"`
		Exp           int64       `json:"exp"`
		Iat           int64       `json:"iat"`
		Jti           string      `json:"jti"`
	}
	MatchId                 string   `json:"matchId"`
	SessionId               string   `json:"sessionId"`
	IsAssigned              bool     `json:"isAssigned"`
	IsAssigning             bool     `json:"isAssigning"`
	StopAllowingConnections bool     `json:"stopAllowingConnections"`
	Playlist                string   `json:"playlist"`
	Teams                   []string `json:"teams"`
	IsSending               bool     `json:"isSending"`
	AssignMatchSent         bool     `json:"assignMatchSent"`
	MinPlayers              int      `json:"minPlayers"`
	MaxPlayers              int      `json:"maxPlayers"`
}

var (
	Sessions = make(map[string]*Server)
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

	server := &Server{
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
	server.Teams = make([]string, 0)
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

	go func() {
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				utils.LogError("failed to read message: %v", err)
				break
			}
			var data map[string]interface{}
			if err := json.Unmarshal(message, &data); err != nil {
				utils.LogError("failed to decode message: %v", err)
				ws.Close()
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
						go func(s *Server) {
							time.Sleep(2 * time.Second)
							s.IsAssigned = true
						}(server)
						go func(s *Server) {
							time.Sleep(3 * time.Second)
							s.StopAllowingConnections = true
						}(server)
					}
				}
			}
		}
	}()

	defer func() {
		ws.Close()
		delete(Sessions, server.SessionId)
	}()

	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if err := ws.WriteMessage(websocket.PongMessage, nil); err != nil {
				utils.LogError("failed to send ping: %v", err)
				ws.Close()
				return
			}
		}
	}()
}
