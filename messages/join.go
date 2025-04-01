package messages

import (
	"vesta/utils"

	"github.com/gorilla/websocket"
)

func SendJoin(ws *websocket.Conn, matchID, sessionID string) error {
	msg := map[string]interface{}{
		"payload": map[string]interface{}{
			"matchId":      matchID,
			"sessionId":    sessionID,
			"joinDelaySec": 1,
		},
		"name": "Play",
	}
	return utils.SendMessage(ws, msg)
}
