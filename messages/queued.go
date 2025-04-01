package messages

import (
	"vesta/utils"

	"github.com/gorilla/websocket"
)

func SendQueued(ws *websocket.Conn, ticketID string, clients int) error {
	msg := map[string]interface{}{
		"payload": map[string]interface{}{
			"ticketId":         ticketID,
			"queuedPlayers":    clients,
			"estimatedWaitSec": 0,
			"status":           map[string]interface{}{},
			"state":            "Queued",
		},
		"name": "StatusUpdate",
	}
	return utils.SendMessage(ws, msg)
}
