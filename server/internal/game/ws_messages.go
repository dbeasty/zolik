package game

import "encoding/json"

type WSIncoming struct {
	Type     string   `json:"type"`
	From     string   `json:"from,omitempty"`
	Cards    []string `json:"cards,omitempty"`
	MeldID   string   `json:"meldId,omitempty"`
	Card     string   `json:"card,omitempty"`
	Position string   `json:"position,omitempty"`
}

func DecodeIncoming(data []byte) (WSIncoming, error) {
	var in WSIncoming
	err := json.Unmarshal(data, &in)
	return in, err
}
