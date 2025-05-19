package handlers

import (
	"github.com/gorilla/websocket"
)

func GetAllClients() []*websocket.Conn {
	ClientM.RLock()
	defer ClientM.RUnlock()

	var connList []*websocket.Conn
	for client := range Clients {
		connList = append(connList, client.Conn)
	}
	return connList
}

func GetAllClientsViaData(version string, playlist string, region string) []*Client {
	ClientM.RLock()
	defer ClientM.RUnlock()

	var connList []*Client
	for client := range Clients {
		if client.Payload.Version == version && client.Payload.Playlist == playlist && client.Payload.Region == region {
			connList = append(connList, client)
		}
	}

	return connList
}

func GetAllClientsViaDataLen(version string, playlist string, region string) int {
	ClientM.RLock()
	defer ClientM.RUnlock()

	var connList []*Client
	for client := range Clients {
		if client.Payload.Version == version && client.Payload.Playlist == playlist && client.Payload.Region == region {
			connList = append(connList, client)
		}
	}

	return len(connList)
}
