package classes

type AssignMatchPayload struct {
	BucketId       string                 `json:"bucketId"`
	MatchId        string                 `json:"matchId"`
	MatchOptions   string                 `json:"matchOptions"`
	MatchOptionsV2 map[string]interface{} `json:"matchOptionsV2"`
	Spectators     []interface{}          `json:"spectators"`
	Teams          []interface{}          `json:"teams"`
}
