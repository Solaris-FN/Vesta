package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"vesta/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	Conn    *websocket.Conn
	Payload struct {
		AccountID      string `json:"accountId"`
		BucketID       string `json:"bucketId"`
		BuildUniqueID  string `json:"buildUniqueId"`
		Exp            int64  `json:"exp"`
		FillTeam       string `json:"fillTeam"`
		Iat            int64  `json:"iat"`
		Jti            string `json:"jti"`
		PartyPlayerIDs string `json:"partyPlayerIds"`
		Playlist       string `json:"playlist"`
		Region         string `json:"region"`
		Version        string `json:"version"`
	}
}

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	clients = make(map[*Client]bool)
	clientM sync.RWMutex
)

func HandleWebSocket(c *gin.Context) {
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
	if len(authParts) != 4 || authParts[0] != "Epic-Signed" || authParts[1] != "Vesta" {
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

	client := &Client{
		Conn: ws,
	}

	if bucketID, ok := payload["bucketId"].(string); ok {
		client.Payload.BucketID = bucketID
	}

	if buildUniqueID, ok := payload["buildUniqueId"].(string); ok {
		client.Payload.BuildUniqueID = buildUniqueID
	}

	if exp, ok := payload["exp"].(float64); ok {
		client.Payload.Exp = int64(exp)
	}

	if fillTeam, ok := payload["fillTeam"].(string); ok {
		client.Payload.FillTeam = fillTeam
	}

	if iat, ok := payload["iat"].(float64); ok {
		client.Payload.Iat = int64(iat)
	}

	if jti, ok := payload["jti"].(string); ok {
		client.Payload.Jti = jti
	}

	if partyPlayerIDs, ok := payload["partyPlayerIds"].(string); ok {
		client.Payload.PartyPlayerIDs = partyPlayerIDs
	}

	if playlist, ok := payload["playlist"].(string); ok {
		client.Payload.Playlist = playlist
	}

	if region, ok := payload["region"].(string); ok {
		client.Payload.Region = region
	}

	if version, ok := payload["version"].(string); ok {
		client.Payload.Version = version
	}

	if accountID, ok := payload["accountId"].(string); ok {
		client.Payload.AccountID = accountID
	}

	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	clientM.Lock()
	clients[client] = true
	clientM.Unlock()
	currentCount := GetAllClientsViaDataLen(client.Payload.Version, client.Payload.Playlist, client.Payload.Region)

	utils.LogSuccess("%s", fmt.Sprintf("Connection established from %s! Current count: %d", r.RemoteAddr, currentCount))

	ticketID := strings.ReplaceAll(uuid.New().String(), "-", "")

	if err := sendInitMessages(ws, ticketID, currentCount); err != nil {
		log.Printf("Failed to send init messages: %v", err)
		clientM.Lock()
		delete(clients, client)
		clientM.Unlock()
		ws.Close()
		return
	}

	if err := HandleStates(*client, ticketID); err != nil {
		log.Printf("HandleStates failed: %v", err)
	}

	clientM.Lock()
	delete(clients, client)
	clientM.Unlock()
	ws.Close()

	utils.LogInfo("%s", fmt.Sprintf("Client disconnected from %s!", r.RemoteAddr))
}
