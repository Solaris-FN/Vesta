package handlers

import (
	"strings"
	"vesta/database"
	"vesta/database/entities"
	"vesta/messages"
	"vesta/utils"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

var lastSelectedPlaylist = make(map[string]string)

func HandlePlaylistSelection(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		return
	}

	db := database.Get()
	var session entities.Session
	if err := db.Where("session = ?", id).First(&session).Error; err != nil {
		c.JSON(404, gin.H{
			"err": "Session not found",
		})
		return
	}

	var sessions []entities.Session
	if err := db.Where("region = ?", session.Region).Find(&sessions).Error; err != nil {
		c.JSON(500, gin.H{
			"err": "Failed to get sessions",
		})
		return
	}

	if _, exists := lastSelectedPlaylist[session.Region]; !exists {
		lastSelectedPlaylist[session.Region] = "playlist_showdownalt_solo"
	}

	if lastSelectedPlaylist[session.Region] == "playlist_showdownalt_duos" {
		lastSelectedPlaylist[session.Region] = "playlist_showdownalt_solo"
	}

	clientM.RLock()
	defer clientM.RUnlock()

	playerCounts := make(map[string]int)
	for client := range clients {
		if client.Payload.Region == session.Region {
			playerCounts[client.Payload.Playlist] = playerCounts[client.Payload.Playlist] + 1
		}
	}

	if len(playerCounts) == 0 {
		c.JSON(200, gin.H{
			"Playlist": nil,
			"Status":   "WAITING",
		})
		return
	}

	serverCounts := make(map[string]int)
	for _, session := range sessions {
		serverCounts[session.PlaylistName] = serverCounts[session.PlaylistName] + 1
	}

	type PlaylistMetric struct {
		Playlist         string
		PlayerCount      int
		ServerCount      int
		PlayersPerServer float64
		NeedsServer      bool
	}

	var metrics []PlaylistMetric
	for playlist, playerCount := range playerCounts {
		serverCount := serverCounts[playlist]
		var playersPerServer float64
		if serverCount > 0 {
			playersPerServer = float64(playerCount) / float64(serverCount)
		} else {
			playersPerServer = float64(playerCount)
		}

		maxPlayersPerServer := 50
		if isDefaultPlaylist(playlist) {
			maxPlayersPerServer = 100
		}

		needsServer := playersPerServer >= float64(maxPlayersPerServer) || serverCount == 0

		metrics = append(metrics, PlaylistMetric{
			Playlist:         playlist,
			PlayerCount:      playerCount,
			ServerCount:      serverCount,
			PlayersPerServer: playersPerServer,
			NeedsServer:      needsServer,
		})
	}

	for _, metric := range metrics {
		if metric.NeedsServer {
			lastSelectedPlaylist[session.Region] = metric.Playlist

			session.PlaylistName = metric.Playlist
			db.Save(&session)

			for _, client := range GetAllClientsViaData(
				session.Version,
				metric.Playlist,
				session.Region,
			) {
				newPlayer := entities.Player{
					AccountID: client.Payload.AccountID,
					Session:   session.Session,
					Team:      pq.StringArray(strings.Split(client.Payload.PartyPlayerIDs, ",")),
				}
				if err := db.Create(&newPlayer).Error; err != nil {
					utils.LogError("Failed to create player: %v", err)
				}

				if err := messages.SendSessionAssignment(client.Conn, id); err != nil {
					utils.LogError("Failed to send session assignment: %v", err)
				}
			}

			c.JSON(200, gin.H{
				"Playlist": metric.Playlist,
				"Status":   "OK",
			})
			return
		}
	}

	c.JSON(200, gin.H{
		"Playlist": nil,
		"Status":   "WAITING",
	})
}

func isDefaultPlaylist(playlist string) bool {
	return contains(playlist, "playlist_default")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr
}
