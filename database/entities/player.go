package entities

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Player struct {
	gorm.Model
	AccountID string         `gorm:"column:account_id;primaryKey"`
	Session   string         `gorm:"column:session"`
	Team      pq.StringArray `gorm:"column:team;type:text[]"`
}

func (Player) TableName() string {
	return "vesta_players"
}
