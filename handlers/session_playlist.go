package handlers

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
	"vesta/database"
	"vesta/database/entities"
	"vesta/messages"
	"vesta/utils"
)

func SelectPlaylist(sessionID string, region string) (string, string, error) {
	if sessionID == "" {
		return "", "WAITING", fmt.Errorf("empty session ID")
	}

	db := database.Get()
	var session entities.MMSessions
	if err := db.Where("session_id = ?", sessionID).First(&session).Error; err != nil {
		return "", "WAITING", fmt.Errorf("session not found: %w", err)
	}

	if region == "" {
		region = session.Region
	}

	var sessions []entities.Session
	if err := db.Where("region = ?", region).Find(&sessions).Error; err != nil {
		return "", "WAITING", fmt.Errorf("failed to get sessions for region: %w", err)
	}

	playlistMutex.Lock()
	if _, exists := lastSelectedPlaylist[region]; !exists {
		lastSelectedPlaylist[region] = "playlist_showdownalt_solo"
	}
	if lastSelectedPlaylist[region] == "playlist_showdownalt_duos" {
		lastSelectedPlaylist[region] = "playlist_showdownalt_solo"
	}
	playlistMutex.Unlock()

	clientM.RLock()
	playerCounts := make(map[string]int)
	for client := range clients {
		if client.Payload.Region == region {
			playerCounts[client.Payload.Playlist]++
		}
	}
	clientM.RUnlock()

	if len(playerCounts) == 0 {
		return "", "WAITING", nil
	}

	serverCounts := make(map[string]int)
	for _, s := range sessions {
		serverCounts[s.PlaylistName]++
	}

	var attributes map[string]interface{}
	if err := json.Unmarshal([]byte(session.Attributes), &attributes); err != nil {
		utils.LogError("failed to unmarshal session attributes: %v", err)
		return "", "WAITING", fmt.Errorf("failed to get session attributes: %w", err)
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

		for _, client := range GetAllClientsViaData(Sessions[session.SessionId].Payload.Version, metric.Playlist, region) {
			if err := messages.SendSessionAssignment(client.Conn, sessionID); err != nil {
				utils.LogError("failed to send session assignment: %v", err)
			}
		}

		return metric.Playlist, "OK", nil
	}

	return "", "WAITING", nil
}

