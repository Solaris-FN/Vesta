package main

import (
	"encoding/json"
	"log"
	"os"
	"time"
	"vesta/classes"
	"vesta/database"
	"vesta/database/entities"
	"vesta/handlers"
	managers "vesta/manager"
	"vesta/utils"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
)

func main() {
	log.SetFlags(0)
	db, err := database.Init()
	if err != nil {
		color.Red("Failed to connect to database: %v", err)
	}
	gin.SetMode(gin.ReleaseMode)
	db.AutoMigrate(&entities.Session{}, &entities.Player{}, &entities.MMSessions{})

	configFile, err := os.Open("./static/config.json")
	if err != nil {
		color.Red("Failed to read config: %v", err)
	}
	defer configFile.Close()

	decoder := json.NewDecoder(configFile)
	if err := decoder.Decode(&classes.Config); err != nil {
		color.Red("Failed to decode config: %v", err)
	}

	go cleanup()

	var router *gin.Engine
	if verbose, ok := classes.Config["Verbose"].(bool); ok && verbose {
		router = gin.Default()
	} else {
		router = gin.New()
	}

	router.GET("/vesta/conn", handlers.HandleWebSocket)
	router.GET("/vesta/session", handlers.HandleSessionWebSocket)
	router.GET("/vesta/queue", managers.GetQueuedPlayersTotal)

	Session := router.Group("/solaris/api/server")
	{
		Session.POST("/session", managers.PostCreateSession)
		Session.GET("/session/:id/playlist", handlers.HandlePlaylistSelection)
		Session.GET("/session/:id/:accountId/player", managers.GetPlayerInSession)
		Session.POST("/session/:id/start", managers.PostStartSession)
		Session.POST("/session/:id/close", managers.PostCloseSession)
		Session.POST("/session/:id/heartbeat", managers.PostSessionHeartbeat)
		Session.DELETE("/session/:id", managers.DeleteSession)
	}

	serverAddr := ":8443"
	utils.LogWithTimestamp(color.BlueString, "%s", "Vesta started on port "+serverAddr)
	go func() {
		if err := router.RunTLS(serverAddr, "static/RootCA.key", "static/RootCA.pem"); err != nil {
			utils.LogWithTimestamp(color.RedString, "Error starting TLS server: %v", err)
		}
	}()
	if err := router.Run(":21921"); err != nil {
		utils.LogWithTimestamp(color.RedString, "Error starting HTTP server: %v", err)
	}
}

func cleanup() {
	db := database.Get()
	utils.LogWithTimestamp(color.GreenString, "Starting session cleanup")
	playerDiffMap := make(map[string]int)
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		utils.LogWithTimestamp(color.YellowString, "Running cleanup check")
		var sessions []entities.MMSessions
		if err := db.Find(&sessions).Error; err != nil {
			utils.LogWithTimestamp(color.RedString, "Error fetching sessions: %v", err)
			continue
		}

		utils.LogWithTimestamp(color.YellowString, "Found %d sessions in database", len(sessions))
		cleanupCount := 0

		for _, session := range sessions {
			utils.LogWithTimestamp(color.YellowString, "Checking session %s (players: %d)",
				session.SessionId, len(session.PublicPlayers))

			lastUpdatedTime, err := time.Parse(time.RFC3339, session.LastUpdated)
			if err != nil {
				utils.LogWithTimestamp(color.RedString, "Error parsing LastUpdated for session %s: %v", session.SessionId, err)
				continue
			}

			var foundServer *classes.Server
			for _, server := range handlers.Sessions {
				if server.SessionId == session.SessionId {
					foundServer = server
					break
				}
			}

			if time.Since(lastUpdatedTime) > 30*time.Minute {
				utils.LogWithTimestamp(color.YellowString, "Session %s is older than 10 minutes, cleaning up", session.SessionId)
				if foundServer != nil {
					foundServer.Conn.Close()
				}
				if err := db.Exec("DELETE FROM mmsessions WHERE session_id = ?", session.SessionId).Error; err != nil {
					utils.LogWithTimestamp(color.RedString, "Error deleting old session %s: %v", session.SessionId, err)
				} else {
					cleanupCount++
				}
				continue
			}

			if len(session.PublicPlayers) == 0 {
				if foundServer == nil {
					utils.LogWithTimestamp(color.YellowString, "Session %s has 0 players and no active connection, deleting", session.SessionId)
					if err := db.Exec("DELETE FROM mmsessions WHERE session_id = ?", session.SessionId).Error; err != nil {
						utils.LogWithTimestamp(color.RedString, "Error deleting empty session %s: %v", session.SessionId, err)
					} else {
						cleanupCount++
					}
				} else {
					utils.LogWithTimestamp(color.YellowString, "Session %s has 0 players but active connection, keeping", session.SessionId)
				}
				continue
			}

			if previousPlayerCount, exists := playerDiffMap[session.SessionId]; exists {
				utils.LogWithTimestamp(color.YellowString, "Previous player count for %s: %d, current: %d",
					session.SessionId, previousPlayerCount, len(session.PublicPlayers))

				if foundServer != nil {
					utils.LogWithTimestamp(color.YellowString, "Found server for session %s (teams: %d)",
						session.SessionId, len(foundServer.Teams))

					shouldDelete := (previousPlayerCount == len(session.PublicPlayers)) &&
						time.Since(lastUpdatedTime) > 30*time.Minute

					utils.LogWithTimestamp(color.YellowString, "Should delete session %s: %v", session.SessionId, shouldDelete)

					if shouldDelete {
						utils.LogWithTimestamp(color.YellowString, "Deleting stagnant session: %s (players: %d)",
							session.SessionId, len(session.PublicPlayers))
						if foundServer.Conn != nil {
							foundServer.Conn.Close()
						}
						if err := db.Exec("DELETE FROM mmsessions WHERE session_id = ?", session.SessionId).Error; err != nil {
							utils.LogWithTimestamp(color.RedString, "Error deleting session %s: %v", session.SessionId, err)
						} else {
							cleanupCount++
						}
					}
				} else {
					utils.LogWithTimestamp(color.YellowString, "Session %s has players but no server connection", session.SessionId)
					if time.Since(lastUpdatedTime) > 3*time.Minute {
						utils.LogWithTimestamp(color.YellowString, "Cleaning up orphaned session %s", session.SessionId)
						if err := db.Exec("DELETE FROM mmsessions WHERE session_id = ?", session.SessionId).Error; err != nil {
							utils.LogWithTimestamp(color.RedString, "Error deleting orphaned session %s: %v", session.SessionId, err)
						} else {
							cleanupCount++
							delete(handlers.Sessions, session.SessionId)
						}
					}
				}
			} else {
				utils.LogWithTimestamp(color.YellowString, "No previous data for session %s", session.SessionId)
			}

			playerDiffMap[session.SessionId] = len(session.PublicPlayers)
		}

		if cleanupCount > 0 {
			utils.LogWithTimestamp(color.GreenString, "Cleaned up %d sessions", cleanupCount)
		} else {
			utils.LogWithTimestamp(color.GreenString, "No sessions found to delete!")
		}
	}
}
