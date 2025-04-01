package messages

import (
	"vesta/utils"

	"github.com/gorilla/websocket"
)

func SendConnecting(ws *websocket.Conn) error {
	msg := map[string]interface{}{
		"payload": map[string]interface{}{
			"state": "Connecting",
		},
		"name": "StatusUpdate",
	}
	return utils.SendMessage(ws, msg)
}
