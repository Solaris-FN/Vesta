package managers

import (
	"encoding/json"
	"log"
	"strings"
	"vesta/database"
	"vesta/database/entities"
	"vesta/handlers"
	"vesta/messages"
	"vesta/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func PostCreateSession(c *gin.Context) {
	var body struct {
		Playlist      string `json:"Playlist"`
		ServerAddr    string `json:"ServerAddr"`
		ServerPort    int    `json:"ServerPort"`
		ActivePlayers int    `json:"ActivePlayers"`
		AllPlayers    int    `json:"AllPlayers"`
		Region        string `json:"Region"`
		Secret        string `json:"Secret"`
		Attributes    struct {
			Type               string `json:"Type"`
			BLimitedTimeMode   bool   `json:"bLimitedTimeMode"`
			RatingType         string `json:"RatingType"`
			MaxPlayers         int    `json:"MaxPlayers"`
			MaxTeamCount       int    `json:"MaxTeamCount"`
			MaxTeamSize        int    `json:"MaxTeamSize"`
			MaxSocialPartySize int    `json:"MaxSocialPartySize"`
			MaxSquadSize       int    `json:"MaxSquadSize"`
		} `json:"Attributes"`
		JoinInProgress bool   `json:"JoinInProgress"`
		Stats          bool   `json:"Stats"`
		Version        string `json:"Version"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"err": err.Error()})
		return
	}

	db := database.Get()

	newSession := entities.Session{
		Session:       strings.ReplaceAll(uuid.New().String(), "-", ""),
		PlaylistName:  body.Playlist,
		ServerAddr:    body.ServerAddr,
		ServerPort:    body.ServerPort,
		ActivePlayers: body.ActivePlayers,
		AllPlayers:    body.AllPlayers,
		Region:        body.Region,
		Secret:        body.Secret,
		Teams:         pq.StringArray{},
		Attributes: func() string {
			attributes, err := json.Marshal(body.Attributes)
			if err != nil {
				c.JSON(404, gin.H{"err": "Failed to marshal attributes"})
				return ""
			}
			return string(attributes)
		}(),
		JoinInProgress: body.JoinInProgress,
		Stats:          body.Stats,
		Available:      false,
		Version:        body.Version,
	}

	db.Create(&newSession)

	for _, client := range handlers.GetAllClientsViaData(body.Version, body.Playlist, body.Region) {
		if err := messages.SendSessionAssignment(client.Conn, newSession.Session); err != nil {
			utils.LogError("Failed to send session assignment: %v", err)
		}
	}

	c.JSON(200, &newSession)
}

func PostStartSession(c *gin.Context) {
	id := c.Param("id")

	db := database.Get()

	var session entities.Session
	if err := db.Where("session = ?", id).First(&session).Error; err != nil {
		c.JSON(404, gin.H{"err": "Session not found"})
		return
	}

	session.Available = true
	db.Save(&session)

	for _, client := range handlers.GetAllClientsViaData(session.Version, session.PlaylistName, session.Region) {
		log.Printf("Sending session assignment to client: %s", client.Conn.RemoteAddr())
		if err := messages.SendJoin(client.Conn, session.Session, session.Session); err != nil {
			utils.LogError("Failed to send join: %v", err)
		}
	}

	c.JSON(200, &session)
}

func PostCloseSession(c *gin.Context) {
	id := c.Param("id")

	db := database.Get()

	var session entities.Session
	if err := db.Where("session = ?", id).First(&session).Error; err != nil {
		c.JSON(404, gin.H{"err": "Session not found"})
		return
	}

	session.Available = false
	db.Save(&session)

	c.JSON(200, &session)
}

func DeleteSession(c *gin.Context) {
	id := c.Param("id")

	db := database.Get()

	var session entities.Session
	if err := db.Where("session = ?", id).First(&session).Error; err != nil {
		c.JSON(404, gin.H{"err": "Session not found"})
		return
	}

	db.Delete(&session)

	c.JSON(204, nil)
}
