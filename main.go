package main

import (
	"io"
	"log"
	"net/http"

	"vesta/handlers"
	"vesta/utils"

	"github.com/fatih/color"
)

func main() {
	http.HandleFunc("/vesta/conn", handlers.HandleWebSocket)

	server := &http.Server{
		Addr:     ":8443",
		ErrorLog: log.New(io.Discard, "", 0),
	}

	utils.LogWithTimestamp(color.BlueString, true, "Vesta started on port :8443")

	if err := server.ListenAndServe(); err != nil {
		utils.LogWithTimestamp(color.RedString, false, "Error starting server: %v", err)
	}
}
