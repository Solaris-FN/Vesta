package entities

import (
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type MMSessions struct {
	gorm.Model
	SessionId                       string         `gorm:"column:session_id"`
	PlaylistName                    string         `gorm:"column:playlist_name"`
	ServerAddress                   string         `gorm:"column:server_address"`
	LastUpdated                     string         `gorm:"column:last_updated"`
	OwnerId                         string         `gorm:"column:owner_id"`
	OwnerName                       string         `gorm:"column:owner_name"`
	ServerName                      string         `gorm:"column:server_name"`
	MaxPublicPlayers                int            `gorm:"column:max_public_players"`
	MaxPrivatePlayers               int            `gorm:"column:max_private_players"`
	ShouldAdvertise                 bool           `gorm:"column:should_advertise"`
	AllowJoinInProgress             bool           `gorm:"column:allow_join_in_progress"`
	IsDedicated                     bool           `gorm:"column:is_dedicated"`
	UsesStats                       bool           `gorm:"column:uses_stats"`
	AllowInvites                    bool           `gorm:"column:allow_invites"`
	UsesPresence                    bool           `gorm:"column:uses_presence"`
	AllowJoinViaPresence            bool           `gorm:"column:allow_join_via_presence"`
	AllowJoinViaPresenceFriendsOnly bool           `gorm:"column:allow_join_via_presence_friends_only"`
	BuildUniqueId                   string         `gorm:"column:build_unique_id"`
	Attributes                      string         `gorm:"column:attributes"`
	ServerPort                      int            `gorm:"column:server_port"`
	OpenPublicPlayers               int            `gorm:"column:open_public_players"`
	OpenPrivatePlayers              int            `gorm:"column:open_private_players"`
	SortWeight                      int            `gorm:"column:sort_weight"`
	Started                         bool           `gorm:"column:started"`
	PublicPlayers                   pq.StringArray `gorm:"column:public_players;type:text[]"`
	PrivatePlayers                  pq.StringArray `gorm:"column:private_players;type:text[]"`
	Stopped                         bool           `gorm:"column:stopped"`
	Region                          string         `gorm:"column:region"`
}

func (MMSessions) TableName() string {
	return "mmsessions"
}
