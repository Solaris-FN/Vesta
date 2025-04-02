package main

import (
	"io"
	"log"
	"net/http"

	"vesta/database"
	"vesta/database/entities"
	"vesta/handlers"
	"vesta/utils"

	"github.com/fatih/color"
)

func main() {
	db, err := database.Init()
	if err != nil {
		color.Red("Failed to connect to database: %v", err)
		log.Fatalf("Failed to connect to database: %v", err)
	}

	db.AutoMigrate(&entities.Session{})

	http.HandleFunc("/vesta/conn", handlers.HandleWebSocket)

	server := &http.Server{
		Addr:     ":8443",
		ErrorLog: log.New(io.Discard, "", 0),
	}

	utils.LogWithTimestamp(color.BlueString, true, "%s", "Vesta started on port "+server.Addr)

	if err := server.ListenAndServe(); err != nil {
		utils.LogWithTimestamp(color.RedString, false, "Error starting server: %v", err)
	}
}
