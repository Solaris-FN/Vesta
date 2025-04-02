package managers

import (
	"vesta/database"
	"vesta/database/entities"

	"github.com/gin-gonic/gin"
)

func GetPlayerInSession(c *gin.Context) {
	id := c.Param("id")
	accountID := c.Param("accountId")
	if id == "" || accountID == "" {
		c.JSON(400, gin.H{"err": "Session not found"})
		return
	}

	db := database.Get()
	var session entities.Session
	if err := db.Where("session = ?", id).First(&session).Error; err != nil {
		c.JSON(404, gin.H{"err": "Session not found"})
		return
	}

	var player entities.Player
	if err := db.Where("session = ? AND account_id = ?", id, accountID).Find(&player).Error; err != nil {
		c.JSON(500, gin.H{"err": "Failed to get players"})
		return
	}

	c.JSON(200, player)
}
