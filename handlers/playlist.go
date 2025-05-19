package handlers

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"
	"vesta/database"
	"vesta/database/entities"
	"vesta/messages"
	"vesta/utils"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

type PlaylistM struct {
	Sum         int
	Hits        int
	LastUpdated int64
}

var (
	lastSelectedPlaylist = make(map[string]string)
	playlistMutex        sync.RWMutex

	playlistStats      = make(map[string]map[string]*PlaylistM) // region -> playlist -> metrics
	playlistStatsMutex sync.RWMutex
)

func HandlePlaylistSelection(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		return
	}

	db := database.Get()
	var session entities.Session
	if err := db.Where("session = ?", id).First(&session).Error; err != nil {
		c.JSON(404, gin.H{"err": "session not found"})
		return
	}

	region := session.Region

	var sessions []entities.Session
	if err := db.Where("region = ?", region).Find(&sessions).Error; err != nil {
		c.JSON(500, gin.H{"err": "failed to get sessions for region"})
		return
	}

	playlistMutex.Lock()
	if _, exists := lastSelectedPlaylist[region]; !exists {
		lastSelectedPlaylist[region] = "playlist_showdownalt_solo"
	}
	if lastSelectedPlaylist[region] == "playlist_showdownalt_duos" {
		lastSelectedPlaylist[region] = "playlist_showdownalt_solo"
	}
	playlistMutex.Unlock()

	ClientM.RLock()
	defer ClientM.RUnlock()

	playerCounts := make(map[string]int)
	for client := range Clients {
		if client.Payload.Region == region {
			playerCounts[client.Payload.Playlist]++
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
	for _, s := range sessions {
		serverCounts[s.PlaylistName]++
	}

	var attributes map[string]interface{}
	if err := json.Unmarshal([]byte(session.Attributes), &attributes); err != nil {
		utils.LogError("failed to unmarshal session attributes: %v", err)
		c.JSON(500, gin.H{"err": "failed to get session attributes"})
		return
	}

	maxPlayersPerServer := 50
	if maxPlayers, ok := attributes["MaxPlayers"].(float64); ok {
		maxPlayersPerServer = int(maxPlayers)
	}

	type Metric struct {
		Playlist         string
		PlayerCount      int
		ServerCount      int
		PlayersPerServer float64
		NeedsServer      bool
	}

	var (
		metrics   []Metric
		metricsMu sync.Mutex
		wg        sync.WaitGroup
	)

	playlistStatsMutex.Lock()
	if _, ok := playlistStats[region]; !ok {
		playlistStats[region] = make(map[string]*PlaylistM)
	}
	currentTime := time.Now().Unix()
	for playlist, count := range playerCounts {
		if _, ok := playlistStats[region][playlist]; !ok {
			playlistStats[region][playlist] = &PlaylistM{Sum: 0, Hits: 0, LastUpdated: currentTime}
		}
		stats := playlistStats[region][playlist]
		stats.Sum += count
		stats.Hits++
		stats.LastUpdated = currentTime
	}
	playlistStatsMutex.Unlock()

	for playlist, playerCount := range playerCounts {
		wg.Add(1)
		go func(playlist string, playerCount int) {
			defer wg.Done()

			serverCount := serverCounts[playlist]
			playersPerServer := float64(playerCount)
			if serverCount > 0 {
				playersPerServer = float64(playerCount) / float64(serverCount)
			}

			needsServer := playersPerServer >= float64(maxPlayersPerServer) || serverCount == 0

			min := 2
			playlistStatsMutex.RLock()
			if stats, ok := playlistStats[region][playlist]; ok && stats.Hits > 0 {
				if time.Now().Unix()-stats.LastUpdated < 180 {
					average := float64(stats.Sum) / float64(stats.Hits)
					calculated := int(average / 2)
					if calculated > min {
						min = calculated
					}
				}
			}
			playlistStatsMutex.RUnlock()

			if playerCount >= min && needsServer {
				metric := Metric{
					Playlist:         playlist,
					PlayerCount:      playerCount,
					ServerCount:      serverCount,
					PlayersPerServer: playersPerServer,
					NeedsServer:      needsServer,
				}

				metricsMu.Lock()
				metrics = append(metrics, metric)
				metricsMu.Unlock()
			}
		}(playlist, playerCount)
	}

	wg.Wait()

	sort.SliceStable(metrics, func(i, j int) bool {
		if metrics[i].NeedsServer != metrics[j].NeedsServer {
			return metrics[i].NeedsServer
		}
		return metrics[i].PlayerCount > metrics[j].PlayerCount
	})

	for _, metric := range metrics {
		playlistMutex.Lock()
		lastSelectedPlaylist[region] = metric.Playlist
		playlistMutex.Unlock()

		session.PlaylistName = metric.Playlist
		db.Save(&session)

		for _, client := range GetAllClientsViaData(session.Version, metric.Playlist, region) {
			newPlayer := entities.Player{
				AccountID: client.Payload.AccountID,
				Session:   session.Session,
				Team:      pq.StringArray(strings.Split(client.Payload.PartyPlayerIDs, ",")),
			}
			if err := db.Create(&newPlayer).Error; err != nil {
				utils.LogError("failed to create player: %v", err)
			}

			if err := messages.SendSessionAssignment(client.Conn, id); err != nil {
				utils.LogError("failed to send session assignment: %v", err)
			}
		}

		c.JSON(200, gin.H{
			"Playlist": strings.TrimPrefix(metric.Playlist, "playlist_"),
			"Status":   "OK",
		})
		return
	}

	c.JSON(200, gin.H{
		"Playlist": nil,
		"Status":   "WAITING",
	})
}
