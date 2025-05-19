package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
	"vesta/classes"
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

	var dbsessions []entities.Session
	if err := db.Where("region = ?", region).Find(&dbsessions).Error; err != nil {
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

	ClientM.RLock()
	playerCounts := make(map[string]int)
	players := make(map[string]*Client)
	for client := range Clients {
		if client.Payload.Region == region {
			playerCounts[client.Payload.Playlist]++
			players[client.Payload.AccountID] = client
		}
	}
	ClientM.RUnlock()

	if len(playerCounts) == 0 {
		return "", "WAITING", nil
	}

	serverCounts := make(map[string]int)
	for _, s := range dbsessions {
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

			min := 1
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
		for _, player := range players {
			sesh := Sessions[session.SessionId]
			if sesh == nil {
				log.Printf("session not found in memory: %+v", session)
				continue
			}

			if sesh.Teams == nil {
				sesh.Teams = make([][][]string, 0)
			}

			ids := strings.Split(player.Payload.PartyPlayerIDs, ",")

			teamIndex := -1
			for i, team := range sesh.Teams {
				for _, playerEntry := range team {
					for _, existingId := range playerEntry {
						for _, newId := range ids {
							if existingId == newId {
								teamIndex = i
								break
							}
						}
						if teamIndex != -1 {
							break
						}
					}
					if teamIndex != -1 {
						break
					}
				}
				if teamIndex != -1 {
					break
				}
			}

			if teamIndex == -1 {
				teamIndex = len(sesh.Teams)
				sesh.Teams = append(sesh.Teams, make([][]string, 0))
			}

			for _, id := range ids {
				exists := false
				for _, playerEntry := range sesh.Teams[teamIndex] {
					for _, existingId := range playerEntry {
						if existingId == id {
							exists = true
							break
						}
					}
					if exists {
						break
					}
				}

				if !exists {
					playerEntry := []string{id}
					sesh.Teams[teamIndex] = append(sesh.Teams[teamIndex], playerEntry)
				}
			}

			Sessions[session.SessionId] = sesh
		}
		Sessions[sessionID].IsAssigning = true
		time.Sleep(2000 * time.Millisecond)

		payload := classes.AssignMatchPayload{
			Name: "AssignMatch",
			Payload: classes.AssignMatchPayloadData{
				Spectators:     make([]interface{}, 0),
				Teams:          Sessions[sessionID].Teams,
				BucketId:       fmt.Sprintf("Fortnite:Fortnite:%s:0:%s:%s", session.BuildUniqueId, region, metric.Playlist),
				MatchId:        Sessions[sessionID].MatchId,
				MatchOptions:   "",
				MatchOptionsV2: make(map[string]interface{}),
			},
		}

		msg, err := json.Marshal(payload)
		if err != nil {
			log.Printf("failed to marshal AssignMatch payload: %v", err)
		}

		if err := Sessions[sessionID].Conn.WriteMessage(1, msg); err != nil {
			log.Printf("failed to send AssignMatch message: %v", err)
		}

		for _, client := range GetAllClientsViaData(Sessions[session.SessionId].Payload.Version, metric.Playlist, region) {
			if err := messages.SendSessionAssignment(client.Conn, sessionID); err != nil {
				utils.LogError("failed to send session assignment: %v", err)
			}
		}

		return metric.Playlist, "OK", nil
	}

	return "", "WAITING", nil
}

