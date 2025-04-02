package entities

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Session struct {
	gorm.Model
	Session        string         `gorm:"column:session;primaryKey"`
	PlaylistName   string         `gorm:"column:playlist"`
	ServerAddr     string         `gorm:"column:server_addr"`
	ServerPort     string         `gorm:"column:server_port"`
	ActivePlayers  int            `gorm:"column:active_players"`
	AllPlayers     int            `gorm:"column:all_players"`
	Region         string         `gorm:"column:region"`
	Secret         string         `gorm:"column:secret"`
	Teams          pq.StringArray `gorm:"column:teams;type:text[][]"`
	Attributes     string         `gorm:"column:attributes"`
	JoinInProgress bool           `gorm:"column:join_in_progress"`
	Stats          bool           `gorm:"column:stats"`
}

func (Session) TableName() string {
	return "vesta_sessions"
}
