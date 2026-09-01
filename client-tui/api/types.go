package api

// The generic match protocol, as the terminal client sees it.
//
// A transcription of `server/internal/module/protocol.go`, and nothing else.
// There is deliberately not one game's noun in this file: no meld, no suit, no
// canasta, no blind, no pot. A board is zones, a table is seats, and what a
// player may do is offers.
//
// It replaced a `GameState` with twenty-four rummy-named fields — melds,
// meldMeta, roundReqMet, initialMeldMinimum, contract — which could describe
// exactly one game. This one describes any of them, which is why the terminal
// client now plays all four rather than being a second Žolíky client.

// Fact is a labelled value, resolved by the server and rendered by us.
type Fact struct {
	LabelKey string         `json:"labelKey"`
	Value    string         `json:"value,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}

type CardView struct {
	Card string `json:"card"`
}

// Group is cards within a zone that belong together.
type Group struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind,omitempty"`
	Cards     []string `json:"cards"`
	BadgeKeys []string `json:"badgeKeys,omitempty"`
}

// Zone is one area of the board. A hidden zone sends Count and no Cards.
type Zone struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"` // hand | stack | pile | spread
	OwnerID  string     `json:"ownerId,omitempty"`
	LabelKey string     `json:"labelKey,omitempty"`
	Cards    []CardView `json:"cards,omitempty"`
	Count    int        `json:"count"`
	Groups   []Group    `json:"groups,omitempty"`
}

// Seat is one player as the board shows them: whose turn, and their numbers.
type Seat struct {
	PlayerID  string   `json:"playerId"`
	Active    bool     `json:"active,omitempty"`
	LabelKeys []string `json:"labelKeys,omitempty"`
	Facts     []Fact   `json:"facts,omitempty"`
}

type ViewModel struct {
	Zones   []Zone `json:"zones"`
	Seats   []Seat `json:"seats,omitempty"`
	Header  []Fact `json:"header,omitempty"`
	Status  []Fact `json:"status,omitempty"`
	Prompts []Fact `json:"prompts,omitempty"`
}

// ParamSpec is a non-card input an offer needs: a named choice, or a number in
// a range the engine computed.
type ParamSpec struct {
	Name     string `json:"name"`
	Kind     string `json:"kind,omitempty"` // "" | choice | int
	LabelKey string `json:"labelKey"`
	Choices  []struct {
		Value    string `json:"value"`
		LabelKey string `json:"labelKey"`
	} `json:"choices,omitempty"`
	Min     int `json:"min,omitempty"`
	Max     int `json:"max,omitempty"`
	Step    int `json:"step,omitempty"`
	Default int `json:"default,omitempty"`
}

type Selector struct {
	Zone     string   `json:"zone"`
	OwnerID  string   `json:"ownerId,omitempty"`
	MeldID   string   `json:"meldId,omitempty"`
	Cards    []string `json:"cards,omitempty"`
	MinCards int      `json:"minCards,omitempty"`
	MaxCards int      `json:"maxCards,omitempty"`
}

// ActionOffer is one affordance. The server always sends the full set,
// disabled entries included, each with the engine's own reason.
type ActionOffer struct {
	ID      string      `json:"id"`
	Verb    string      `json:"verb"`
	Enabled bool        `json:"enabled"`
	WhyNot  string      `json:"whyNot,omitempty"`
	Source  *Selector   `json:"source,omitempty"`
	Target  *Selector   `json:"target,omitempty"`
	Params  []ParamSpec `json:"params,omitempty"`
	Facts   []Fact      `json:"facts,omitempty"`
	// Composite means the submission is a combination only a person can
	// compose, from the cards Source lists.
	Composite bool `json:"composite,omitempty"`
}

// Standing is one row of a scoreboard, in a shape no game owns.
type Standing struct {
	PlayerID string `json:"playerId"`
	Rank     int    `json:"rank"`
	Score    int    `json:"score"`
	Won      bool   `json:"won,omitempty"`
	LabelKey string `json:"labelKey,omitempty"`
}

type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IsAI bool   `json:"isAI"`
	// Avatar is the face the graphical clients draw for this seat. Carried
	// for parity rather than use: a terminal has no portrait to show, and a
	// field decoded here is one a future pane could reach for without the
	// wire needing to change again.
	Avatar string `json:"avatar,omitempty"`
}

// MatchState is everything a client renders a match from.
type MatchState struct {
	Type            string        `json:"type"`
	MatchID         string        `json:"matchId"`
	ModuleID        string        `json:"moduleId"`
	Variation       string        `json:"variation,omitempty"`
	Status          string        `json:"status"`
	JoinCode        string        `json:"joinCode,omitempty"`
	HostID          string        `json:"hostId,omitempty"`
	WinnerID        string        `json:"winnerId,omitempty"`
	Winners         []string      `json:"winners,omitempty"`
	SuspendedPlayer string        `json:"suspendedPlayer,omitempty"`
	Players         []Player      `json:"players"`
	View            ViewModel     `json:"view"`
	LegalActions    []ActionOffer `json:"legalActions"`
	Standings       []Standing    `json:"standings,omitempty"`
}

// Action is what the client sends back. Built from an offer, never composed
// by hand.
type Action struct {
	OfferID string            `json:"offerId,omitempty"`
	Verb    string            `json:"verb"`
	Cards   []string          `json:"cards,omitempty"`
	Target  string            `json:"target,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
}

// Module is a game's self-description, as /modules reports it.
type Module struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	MinPlayers int    `json:"minPlayers"`
	MaxPlayers int    `json:"maxPlayers"`
	Variations []struct {
		ID       string         `json:"id"`
		Label    string         `json:"label"`
		Summary  []Fact         `json:"summary,omitempty"`
		Defaults map[string]int `json:"defaults,omitempty"`
	} `json:"variations,omitempty"`
	Options []struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Label   string `json:"label"`
		Help    string `json:"help,omitempty"`
		Choices []struct {
			Value int    `json:"value"`
			Label string `json:"label"`
		} `json:"choices"`
	} `json:"options,omitempty"`
}

// SubmissionFor builds the concrete action an offer describes, using only what
// the offer declares.
//
// The terminal twin of the server's module.SubmissionFor and the RN client's
// submissionFor. Returns false when the offer needs a person — a composite
// combination it cannot compose.
func SubmissionFor(o ActionOffer) (Action, bool) {
	if !o.Enabled || o.Composite {
		return Action{}, false
	}
	a := Action{OfferID: o.ID, Verb: o.Verb}
	if o.Source != nil && o.Source.MinCards > 0 {
		if len(o.Source.Cards) < o.Source.MinCards {
			return Action{}, false
		}
		a.Cards = append([]string(nil), o.Source.Cards[:o.Source.MinCards]...)
	}
	if o.Target != nil && o.Target.MeldID != "" {
		a.Target = o.Target.MeldID
	}
	for _, p := range o.Params {
		v, ok := DefaultParam(p)
		if !ok {
			return Action{}, false
		}
		if a.Params == nil {
			a.Params = map[string]string{}
		}
		a.Params[p.Name] = v
	}
	return a, true
}

// DefaultParam is a legal starting value for a declared parameter.
func DefaultParam(p ParamSpec) (string, bool) {
	if p.Kind == "int" {
		v := p.Default
		if v < p.Min || v > p.Max {
			v = p.Min
		}
		return itoa(v), true
	}
	if len(p.Choices) == 0 {
		return "", false
	}
	return p.Choices[0].Value, true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// PlayerName looks a seat's display name up, falling back to the id.
func PlayerName(players []Player, id string) string {
	for _, p := range players {
		if p.ID == id {
			if p.Name != "" {
				return p.Name
			}
			break
		}
	}
	return id
}
