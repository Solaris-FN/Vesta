package classes

import "github.com/gorilla/websocket"

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
	MatchId                 string       `json:"matchId"`
	SessionId               string       `json:"sessionId"`
	IsAssigned              bool         `json:"isAssigned"`
	IsAssigning             bool         `json:"isAssigning"`
	StopAllowingConnections bool         `json:"stopAllowingConnections"`
	Playlist                string       `json:"playlist"`
	Teams                   [][][]string `json:"teams"`
	IsSending               bool         `json:"isSending"`
	AssignMatchSent         bool         `json:"assignMatchSent"`
	MinPlayers              int          `json:"minPlayers"`
	MaxPlayers              int          `json:"maxPlayers"`
}
