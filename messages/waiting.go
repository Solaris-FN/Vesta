package messages

import (
	"vesta/utils"

	"github.com/gorilla/websocket"
)

func SendWaiting(ws *websocket.Conn) error {
	msg := map[string]interface{}{
		"payload": map[string]interface{}{
			"totalPlayers":     1,
			"connectedPlayers": 1,
			"state":            "Waiting",
		},
		"name": "StatusUpdate",
	}
	return utils.SendMessage(ws, msg)
}
