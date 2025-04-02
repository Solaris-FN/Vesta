package entities 

import (
	"gorm.io/gorm"
)

type Player struct {
	gorm.Model
	AccountID        string         `gorm:"column:account_id;primaryKey"`
	Session        string         `gorm:"column:session"`
}

func (Player) TableName() string {
	return "vesta_players"
}