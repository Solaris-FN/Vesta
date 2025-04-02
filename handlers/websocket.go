package handlers

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"vesta/messages"
	"vesta/utils"

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

func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade to WebSocket", http.StatusInternalServerError)
		log.Printf("upgrade failed: %v", err)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Error, Data not signed correctly!", http.StatusUnauthorized)
		ws.Close()
		return
	}

	authParts := strings.SplitN(authHeader, " ", 4)
	if len(authParts) != 4 || authParts[0] != "Epic-Signed" || authParts[1] != "Vesta" {
		http.Error(w, "Error, Data not signed correctly!", http.StatusUnauthorized)
		ws.Close()
		return
	}

	const JWT_SECRET = "vmkt7lob4n0purvn7n96c3tk8vb5o2a4hu1a8fqisa1xx718bx808ns5si1jhm98qlycpzk8us0b57j8gt5td1c42c1us9ww"
	token := authParts[2] + "." + strings.SplitN(authParts[3], " ", 2)[0]

	_, err = utils.VerifyJWT(token, JWT_SECRET)
	if err != nil {
		http.Error(w, "Error, Data not signed correctly!", http.StatusUnauthorized)
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

func sendInitMessages(ws *websocket.Conn, ticketID string, count int) error {
	if err := messages.SendConnecting(ws); err != nil {
		return err
	}
	time.Sleep(800 * time.Millisecond)

	if err := messages.SendWaiting(ws); err != nil {
		return err
	}
	time.Sleep(1000 * time.Millisecond)

	return messages.SendQueued(ws, ticketID, count)
}

func handleConnection(ws *websocket.Conn, ticketId string) error {
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

func getClientCount() int {
	clientM.RLock()
	defer clientM.RUnlock()
	return len(clients)
}
