package mcp

import "encoding/json"

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func obj(required []string, props map[string]any) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

var tools = []tool{
	{"register_agent",
		"Register this client as an AI agent. Call first. Set available=true to be offered to hosts who want an AI agent at their table; leave false to join tables yourself with join_table.",
		obj([]string{"name"}, map[string]any{
			"name":      str("The name shown on your seat."),
			"label":     str("Your client or model, shown beside the name."),
			"available": boolean("Offer yourself to hosts. Default false."),
		})},
	{"set_available",
		"Start or stop being offered to hosts as an available agent.",
		obj([]string{"available"}, map[string]any{"available": boolean("Whether hosts may seat you.")})},
	{"list_tables",
		"The unfinished tables you are seated at, with whose turn it is. Seats a host gave you appear here.",
		obj(nil, map[string]any{})},
	{"join_table",
		"Take a seat at a table that is still in its lobby, by match id or join code.",
		obj([]string{"table"}, map[string]any{"table": str("Match id or join code.")})},
	{"get_state",
		"The table as you may see it: your hand, the board, standings, and legalActions — what you may do now. playable lists a ready-to-send action for each enabled offer (smallest legal amounts); pass one to act, adjusting params within the offer's range if you want.",
		obj([]string{"matchId"}, map[string]any{"matchId": str("Match id.")})},
	{"wait_for_turn",
		"Block up to timeoutSeconds (max 25) until the table is waiting on you or the game ends, then return the state. Call again if it times out. Calling this (or any tool) is what keeps you counted as present.",
		obj([]string{"matchId"}, map[string]any{
			"matchId":        str("Match id."),
			"timeoutSeconds": map[string]any{"type": "integer", "description": "Default 20, max 25."},
		})},
	{"act",
		"Play a move. Use an enabled offer from legalActions: pass its offerId and verb, and any cards, target or params the offer asks for.",
		obj([]string{"matchId", "verb"}, map[string]any{
			"matchId": str("Match id."),
			"offerId": str("The offer's id."),
			"verb":    str("The offer's verb."),
			"cards":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Card codes, when the offer takes cards."},
			"target":  str("Target, when the offer takes one."),
			"params":  map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}, "description": "Offer parameters, such as a bet amount."},
		})},
}

func toolList() json.RawMessage {
	b, _ := json.Marshal(map[string]any{"tools": tools})
	return b
}
