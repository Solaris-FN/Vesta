package handlers

import (
	"log"

	"github.com/gorilla/websocket"
)

func getClientCount() int {
	clientM.RLock()
	defer clientM.RUnlock()
	return len(clients)
}

func GetAllClients() []*websocket.Conn {
	clientM.RLock()
	defer clientM.RUnlock()

	var connList []*websocket.Conn
	for client := range clients {
		connList = append(connList, client.Conn)
	}
	return connList
}

func GetAllClientsViaData(version string, playlist string, region string) []*Client {
	clientM.RLock()
	defer clientM.RUnlock()

	var connList []*Client
	for client := range clients {
		if client.Payload.Version == version && client.Payload.Playlist == playlist && client.Payload.Region == region {
			connList = append(connList, client)
		} else {
			// Debugging: Log why a client was not added
			log.Printf("Client skipped: Version=%s (expected %s), Playlist=%s (expected %s), Region=%s (expected %s)",
				client.Payload.Version, version, client.Payload.Playlist, playlist, client.Payload.Region, region)
		}
	}
	// Debugging: Log the total number of matching clients
	log.Printf("Total matching clients: %d", len(connList))
	return connList
}
