package classes

type AssignMatchPayload struct {
	Name    string                 `json:"name"`
	Payload AssignMatchPayloadData `json:"payload"`
}

type AssignMatchPayloadData struct {
	BucketId       string                 `json:"bucketId"`
	MatchId        string                 `json:"matchId"`
	MatchOptions   string                 `json:"matchOptions"`
	MatchOptionsV2 map[string]interface{} `json:"matchOptionsV2"`
	Spectators     []interface{}          `json:"spectators"`
	Teams          [][][]string           `json:"teams"`
}

type BackfillMatchPayload struct {
	Name    string                   `json:"name"`
	Payload BackfillMatchPayloadData `json:"payload"`
}

type BackfillMatchPayloadData struct {
	Teams           [][][]string           `json:"teams"`
	BackfillId      string                 `json:"backfillId"`
	BackfillOptions map[string]interface{} `json:"backfillOptions"`
}
