package handlers

import (
	"net/http"
	"strings"
	"time"
	"vesta/utils"

	"github.com/gin-gonic/gin"
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
}

func HandleSessionWebSocket(c *gin.Context) {
	w, r := c.Writer, c.Request

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		ws.Close()
		return
	}

	authParts := strings.SplitN(authHeader, " ", 4)
	if len(authParts) != 4 || authParts[0] != "Epic-Signed" || authParts[1] != "Vesta-Sessions" {
		c.AbortWithStatus(http.StatusUnauthorized)
		ws.Close()
		return
	}

	const JWT_SECRET = "vmkt7lob4n0purvn7n96c3tk8vb5o2a4hu1a8fqisa1xx718bx808ns5si1jhm98qlycpzk8us0b57j8gt5td1c42c1us9ww"
	token := authParts[2] + "." + strings.SplitN(authParts[3], " ", 2)[0]

	payload, err := utils.VerifyJWT(token, JWT_SECRET)
	if err != nil {
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

	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

}
