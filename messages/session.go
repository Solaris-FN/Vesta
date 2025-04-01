package messages

import (
	"vesta/utils"

	"github.com/gorilla/websocket"
)

func SendSessionAssignment(ws *websocket.Conn, matchID string) error {
	msg := map[string]interface{}{
		"payload": map[string]interface{}{
			"matchId": matchID,
			"state":   "SessionAssignment",
		},
		"name": "StatusUpdate",
	}
	return utils.SendMessage(ws, msg)
}
