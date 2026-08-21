package game

import "encoding/json"

type WSIncoming struct {
	Type     string   `json:"type"`
	From     string   `json:"from,omitempty"`
	Cards    []string `json:"cards,omitempty"`
	MeldID   string   `json:"meldId,omitempty"`
	Card     string   `json:"card,omitempty"`
	Position string   `json:"position,omitempty"`
	// CardIndex disambiguates which physical hand card Card refers to when
	// duplicate-value cards (two decks in play) make the value alone
	// ambiguous — the hand-slot index the client believes Card sits at.
	// Optional: nil means "first matching card", same as before this field
	// existed.
	CardIndex *int `json:"cardIndex,omitempty"`
}

func DecodeIncoming(data []byte) (WSIncoming, error) {
	var in WSIncoming
	err := json.Unmarshal(data, &in)
	return in, err
}
