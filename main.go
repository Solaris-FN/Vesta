package main

import (
	"log"
	"time"
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
	db.AutoMigrate(&entities.Session{}, &entities.Player{})

	go cleanup()

	router := gin.New() // use gin.Default() if you want a more verbose vesta server
	router.GET("/vesta/conn", handlers.HandleWebSocket)

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

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		utils.LogWithTimestamp(color.YellowString, "Running cleanup check")

		var sessions []entities.Session
		if err := db.Where("accessible = ?", false).Find(&sessions).Error; err != nil {
			utils.LogWithTimestamp(color.RedString, "Error fetching sessions: %v", err)
			continue
		}

		currentTime := time.Now()
		cleanupCount := 0

		for _, session := range sessions {
			if currentTime.Sub(session.UpdatedAt) > 60*time.Second {
				utils.LogWithTimestamp(color.YellowString, "Deleting session: %s (last active: %v)",
					session.ID, session.UpdatedAt)
				if err := db.Exec("DELETE FROM vesta_sessions WHERE session = ? ", session.ID).Error; err != nil {
					utils.LogWithTimestamp(color.RedString, "Error deleting session %s: %v", session.ID, err)
				} else {
					cleanupCount++
				}
			}
		}

		if cleanupCount > 0 {
			utils.LogWithTimestamp(color.GreenString, "Cleaned up %d sessions", cleanupCount)
		} else {
			utils.LogWithTimestamp(color.GreenString, "No sessions found to delete!")
		}
	}
}
