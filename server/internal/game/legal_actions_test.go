package game

import (
	"encoding/json"
	"testing"

	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// Phase 1 wire tests. The rules package proves the offers are *correct*
// (offers_agreement_test.go); these prove they survive the trip to the
// client intact, are scoped to the viewer, and that the legacy flags they
// replace are genuinely derived from them rather than recomputed.

func offerGame(mut func(*models.Game)) models.Game {
	g := models.Game{
		Status:        "active",
		GameNumber:    1,
		Round:         3, // past continental's discard lock
		Phase:         "meld",
		CurrentTurn:   "p1",
		TurnOrder:     []string{"p1", "p2"},
		DealStarterID: "p1",
		RulesProfile:  "continental",
		DrawPile:      []string{"2C", "3C", "4C"},
		DiscardPile:   []string{"9C", "TC"},
		Players: []models.Player{
			{ID: "p1", Name: "A"},
			{ID: "p2", Name: "B"},
		},
		Hands: map[string][]string{
			"p1": {"4H", "9H", "KS", "KD"},
			"p2": {"2D", "3D"},
		},
		Melds: map[string][][]string{
			"p2": {{"5H", "6H", "7H", "8H"}},
		},
		MeldMeta: map[string][]models.MeldInfo{
			"p2": {{MeldID: "meld_1", Type: "run", OwnerID: "p2"}},
		},
		RoundReqMet: map[string]bool{"p1": true},
		TotalScores: map[string]int{},
		NextMeldSeq: 1,
	}
	cfg := rules.ProfileContinental
	cfg.InitialMeldMinimum = 0
	setGameRules(&g, cfg)
	if mut != nil {
		mut(&g)
	}
	return g
}

func findMsgOffer(t *testing.T, msg GameStateMsg, id string) rules.ActionOffer {
	t.Helper()
	o := rules.FindOffer(msg.LegalActions, id)
	if o == nil {
		t.Fatalf("offer %q missing from the state message", id)
	}
	return *o
}

func TestBuildGameStateMsg_ShipsTheOfferList(t *testing.T) {
	msg := BuildGameStateMsg(offerGame(nil), "p1")
	if len(msg.LegalActions) == 0 {
		t.Fatal("no legal actions on the wire — every client would fall back to guessing")
	}
	// Spot-check the offers that replace named client expressions.
	if o := findMsgOffer(t, msg, rules.LayOffOfferID("meld_1")); !o.Enabled {
		t.Errorf("lay-off should be enabled for a player who is down: whyNot=%s", o.WhyNot)
	}
	if o := findMsgOffer(t, msg, rules.OfferDiscard); !o.Enabled {
		t.Errorf("discard should be enabled in the meld phase: whyNot=%s", o.WhyNot)
	}
}

func TestBuildGameStateMsg_OffersAreScopedToTheViewer(t *testing.T) {
	g := offerGame(nil)
	mine := BuildGameStateMsg(g, "p1")
	theirs := BuildGameStateMsg(g, "p2")

	if o := findMsgOffer(t, theirs, rules.OfferDiscard); o.Enabled {
		t.Error("the player who is not on turn was offered a discard")
	} else if o.WhyNot != rules.ErrNotYourTurn {
		t.Errorf("whyNot = %s, want %s", o.WhyNot, rules.ErrNotYourTurn)
	}

	// The offer list is a new channel out of the server, so it needs the
	// same hidden-information discipline as MyHand/CardCounts: it must never
	// name a card the viewer does not hold.
	for _, msg := range []struct {
		viewer string
		m      GameStateMsg
	}{{"p1", mine}, {"p2", theirs}} {
		hand := map[string]bool{}
		for _, c := range g.Hands[msg.viewer] {
			hand[c] = true
		}
		for _, o := range msg.m.LegalActions {
			if o.Source == nil || o.Source.Zone != rules.ZoneHand {
				continue
			}
			for _, c := range o.Source.Cards {
				if !hand[c] {
					t.Errorf("%s: offer %q leaks card %q not in that viewer's hand", msg.viewer, o.ID, c)
				}
			}
		}
	}
}

func TestBuildGameStateMsg_LegacyUndoFlagsAreDerivedFromOffers(t *testing.T) {
	// These four booleans are kept only for a client build that predates the
	// offer list. If they were recomputed independently they would be four
	// more drift sites; asserting equality here is what makes them a
	// re-spelling rather than a second implementation.
	cases := []struct {
		name string
		mut  func(*models.Game)
	}{
		{"nothing to undo", nil},
		{"after a discard-pile pickup", func(g *models.Game) {
			g.Hands["p1"] = []string{"4H", "9H", "KS", "TC"}
			g.DiscardDrawnCards = []string{"TC"}
		}},
		{"after a lay-off", func(g *models.Game) {
			g.LastLayOff = &models.LayOffSnapshot{
				PlayerID: "p1", MeldID: "meld_1",
				PrevCards: []string{"5H", "6H", "7H", "8H"},
				Cards:     []string{"9H"},
			}
		}},
		{"after a lay-meld", func(g *models.Game) {
			g.LastMeldLaid = &models.MeldLaidSnapshot{
				PlayerID: "p1", MeldID: "meld_1",
				Cards: []string{"5H", "6H", "7H", "8H"},
			}
		}},
		{"mid-turn snapshot", func(g *models.Game) {
			g.TurnMeldSnapshot = &models.TurnMeldSnapshot{
				PlayerID:    "p1",
				Hands:       map[string][]string{"p1": {"4H"}},
				Melds:       map[string][][]string{},
				MeldMeta:    map[string][]models.MeldInfo{},
				DiscardPile: []string{},
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := BuildGameStateMsg(offerGame(tc.mut), "p1")
			for _, pair := range []struct {
				offerID string
				flag    bool
			}{
				{rules.OfferUndoDrawDiscard, msg.CanUndoDiscardDraw},
				{rules.OfferUndoLayOff, msg.CanUndoLayOff},
				{rules.OfferUndoLayMeld, msg.CanUndoLayMeld},
				{rules.OfferUndoTurn, msg.CanUndoTurn},
			} {
				want := findMsgOffer(t, msg, pair.offerID).Enabled
				if pair.flag != want {
					t.Errorf("legacy flag for %q = %v, offer says %v", pair.offerID, pair.flag, want)
				}
			}
		})
	}
}

func TestBuildGameStateMsg_ShipsTheResolvedRulesetAndContract(t *testing.T) {
	// The client no longer switches on the profile *name* to know a
	// profile's constants — which is what makes a third profile a
	// server-only change.
	t.Run("continental deal 2", func(t *testing.T) {
		msg := BuildGameStateMsg(offerGame(func(g *models.Game) { g.GameNumber = 2 }), "p1")
		if msg.Rules.DealSize != 12 || msg.Rules.MinRunSize != 4 {
			t.Errorf("rules = %+v, want continental's 12-card deal / 4-card runs", msg.Rules)
		}
		if msg.Rules.DiscardPickupMode != "top_only" {
			t.Errorf("discardPickupMode = %q, want top_only", msg.Rules.DiscardPickupMode)
		}
		// Continental deal 2 is "one set, one run".
		if msg.Contract != (ContractMsg{Sets: 1, Runs: 1}) {
			t.Errorf("contract = %+v, want {Sets:1 Runs:1}", msg.Contract)
		}
	})

	t.Run("zolik_classic", func(t *testing.T) {
		g := offerGame(func(g *models.Game) {
			g.RulesProfile = "zolik_classic"
			setGameRules(g, rules.ProfileZolikClassic)
		})
		msg := BuildGameStateMsg(g, "p1")
		if msg.Rules.DealSize != 13 || msg.Rules.MinRunSize != 3 {
			t.Errorf("rules = %+v, want zolik_classic's 13-card deal / 3-card runs", msg.Rules)
		}
		if msg.Rules.MatchEndMode != "at_score" || msg.Rules.TargetScore != 200 {
			t.Errorf("match end = %s/%d, want at_score/200", msg.Rules.MatchEndMode, msg.Rules.TargetScore)
		}
		if !msg.Contract.RequireCleanRun {
			t.Error("zolik_classic requires a clean run; contract says otherwise")
		}
	})

	t.Run("a house-rule override reaches the client", func(t *testing.T) {
		// The whole point of persisting the resolved ruleset (Phase 0) is
		// that an override is real. It has to be visible to the UI too,
		// otherwise the rules panel shows the shipped profile's numbers
		// while the engine enforces different ones.
		g := offerGame(func(g *models.Game) {
			cfg := rules.ProfileContinental
			cfg.InitialMeldMinimum = 70
			cfg.DiscardDrawMinRound = 2
			setGameRules(g, cfg)
		})
		msg := BuildGameStateMsg(g, "p1")
		if msg.Rules.InitialMeldMinimum != 70 || msg.Rules.DiscardDrawMinRound != 2 {
			t.Errorf("overrides lost on the wire: %+v", msg.Rules)
		}
		// ...and the deprecated scalars still mirror them.
		if msg.InitialMeldMinimum != 70 || msg.DiscardDrawMinRound != 2 {
			t.Errorf("legacy scalars disagree with rules: %d/%d", msg.InitialMeldMinimum, msg.DiscardDrawMinRound)
		}
	})
}

func TestBuildGameStateMsg_OffersSerializeForTheClient(t *testing.T) {
	// The clients consume this as JSON, so the field names and the
	// omitempty choices are part of the contract. A `whyNot` that
	// serialised as an object, or a missing `legalActions` key, would break
	// them silently.
	msg := BuildGameStateMsg(offerGame(func(g *models.Game) { g.RoundReqMet = map[string]bool{} }), "p1")
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded struct {
		LegalActions []struct {
			ID      string `json:"id"`
			Verb    string `json:"verb"`
			Enabled bool   `json:"enabled"`
			WhyNot  string `json:"whyNot"`
			Source  *struct {
				Zone       string `json:"zone"`
				Cards      []string
				Placements []struct {
					Card      string   `json:"card"`
					Positions []string `json:"positions"`
				} `json:"placements"`
			} `json:"source"`
			Target *struct {
				Zone   string `json:"zone"`
				MeldID string `json:"meldId"`
			} `json:"target"`
		} `json:"legalActions"`
		Rules struct {
			MinRunSize int `json:"minRunSize"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.LegalActions) == 0 {
		t.Fatal("legalActions did not survive JSON round-trip")
	}
	if decoded.Rules.MinRunSize != 4 {
		t.Errorf("rules.minRunSize = %d, want 4", decoded.Rules.MinRunSize)
	}

	var sawDisabledWithReason, sawTargetedOffer bool
	for _, o := range decoded.LegalActions {
		if o.ID == "" || o.Verb == "" {
			t.Errorf("offer with empty id/verb: %+v", o)
		}
		if !o.Enabled && o.WhyNot != "" {
			sawDisabledWithReason = true
		}
		if o.Target != nil && o.Target.MeldID != "" {
			sawTargetedOffer = true
		}
	}
	if !sawDisabledWithReason {
		t.Error("expected at least one disabled offer to carry a whyNot key over the wire")
	}
	if !sawTargetedOffer {
		t.Error("expected a meld-targeted offer to carry its meldId over the wire")
	}
}
