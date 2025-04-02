package handlers

import (
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

var (
	upgrader = websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	clients = make(map[*websocket.Conn]bool)
	clientM sync.RWMutex
)

func HandleWebSocket(c *gin.Context) {
	w, r := c.Writer, c.Request

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		log.Printf("upgrade failed: %v", err)
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
	_, err = utils.VerifyJWT(token, JWT_SECRET)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		ws.Close()
		return
	}

	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	clientM.Lock()
	clients[ws] = true
	currentCount := len(clients)
	clientM.Unlock()

	utils.LogSuccess("connection!")

	ticketID := strings.ReplaceAll(uuid.New().String(), "-", "")
	if err := sendInitMessages(ws, ticketID, currentCount); err != nil {
		clientM.Lock()
		delete(clients, ws)
		clientM.Unlock()
		ws.Close()
		return
	}

	if err := handleConnection(ws, ticketID); err != nil {
		log.Printf("handling failed: %v", err)
	}

	clientM.Lock()
	delete(clients, ws)
	clientM.Unlock()
	ws.Close()
	utils.LogInfo("disconnection!")
}
