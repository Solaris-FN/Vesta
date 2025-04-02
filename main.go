package main

import (
	"log"
	"vesta/database"
	"vesta/database/entities"
	"vesta/handlers"
	managers "vesta/manager"
	"vesta/utils"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
)

func main() {
	db, err := database.Init()
	if err != nil {
		color.Red("Failed to connect to database: %v", err)
		log.Fatalf("Failed to connect to database: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)

	db.AutoMigrate(&entities.Session{})

	router := gin.Default()

	router.GET("/vesta/conn", handlers.HandleWebSocket)

	Session := router.Group("/solaris/api/server")
	{
		Session.POST("/session", managers.PostCreateSession)
		Session.GET("/session/:id/playlist", handlers.HandlePlaylistSelection)
		Session.POST("/session/:id/start", managers.PostStartSession)
		Session.POST("/session/:id/close", managers.PostCloseSession)
		Session.DELETE("/session/:id", managers.DeleteSession)
	}

	serverAddr := ":8443"
	utils.LogWithTimestamp(color.BlueString, true, "%s", "Vesta started on port "+serverAddr)

	if err := router.Run(serverAddr); err != nil {
		utils.LogWithTimestamp(color.RedString, false, "Error starting server: %v", err)
	}
}
