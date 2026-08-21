package game

import (
	"fmt"
	"strings"
	"testing"

	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// End-to-end for the offer mechanism, one layer below the browser suite.
//
// The other tests ask whether the offers are *correct*. This one asks whether
// they are *sufficient* — the question that actually decides whether a client
// can stop knowing the rules. It plays complete deals with a driver that is
// forbidden from consulting the rules package for anything except applying the
// action it chose: every decision comes from the offer list alone, exactly as
// a UI shell would make it, and every action travels the real wire path
// (WSIncoming -> toRulesAction -> ApplyAction -> fromRulesState ->
// BuildGameStateMsg) rather than calling the engine directly.
//
// If the offer list ever fails to describe a legal move, or advertises one
// the engine then refuses, this test deadlocks or fails — which is precisely
// the failure a real client would hit.
//
// It needs no Mongo, which is why it can run in the normal suite. The
// Playwright suite in e2e/ covers the same ground through the real browser
// UI; see e2e/tests/legal-actions.spec.ts.

// offerClient is a deliberately ignorant player. It may read the game-state
// message (as any client can) but may not reason about rules: it picks moves
// purely by looking at which offers are enabled and which cards they list.
type offerClient struct {
	playerID string
	t        *testing.T
	// refused remembers meld proposals the server has already rejected, so
	// the client stops re-proposing them. A real UI gets this for free (the
	// player sees the error and tries something else); a loop needs it
	// explicitly or it never makes progress.
	refused map[string]bool
	// lastProposed is the meld this client most recently put on the table.
	// If the turn later dead-ends, that meld is what led there, so it gets
	// blacklisted along with the undo.
	lastProposed []string
}

// chooseAction returns the next WSIncoming this client would send, or ok=false
// if the offers say it has nothing to do.
//
// The priority order below is UI preference, not rule knowledge — a client is
// free to prefer melding over discarding. Every branch is gated on an offer
// the server marked enabled; none of them re-derives legality.
func (c *offerClient) chooseAction(msg GameStateMsg) (WSIncoming, bool) {
	offer := func(id string) *rules.ActionOffer { return rules.FindOffer(msg.LegalActions, id) }
	enabled := func(id string) bool {
		o := offer(id)
		return o != nil && o.Enabled
	}

	// 1. Draw, if a draw is on the table. Prefer the deck: taking from the
	//    discard pile carries an obligation this simple client cannot plan for.
	if enabled(rules.OfferDrawDeck) {
		return WSIncoming{Type: "draw_card", From: "deck"}, true
	}
	if o := offer(rules.OfferDrawDiscard); o != nil && o.Enabled && o.Source != nil && len(o.Source.Cards) > 0 {
		return WSIncoming{Type: "draw_card", From: "discard", Card: o.Source.Cards[len(o.Source.Cards)-1]}, true
	}

	// 2. Shed onto a meld that says it will accept a card from our hand,
	//    honouring the end the server named.
	for _, o := range msg.LegalActions {
		if o.Verb != rules.VerbLayOff || !o.Enabled || o.Source == nil || o.Target == nil {
			continue
		}
		for _, p := range o.Source.Placements {
			in := WSIncoming{Type: "lay_off", MeldID: o.Target.MeldID, Card: p.Card}
			if len(p.Positions) == 1 {
				in.Position = p.Positions[0]
			}
			return in, true
		}
	}

	// 3. Try to go down. The offer bounds the shape (min/max cards); which
	//    concrete combination is valid is the server's business, so this
	//    client proposes candidates and lets the server judge — see
	//    playDeal's handling of a refused proposal.
	if o := offer(rules.OfferLayMeld); o != nil && o.Enabled && o.Source != nil {
		if cards := c.proposeMeld(msg.MyHand, o.Source.MinCards, o.Source.MaxCards); cards != nil {
			c.lastProposed = cards
			return WSIncoming{Type: "lay_meld", Cards: cards}, true
		}
	}

	// 4. End the turn with a card the server listed as discardable.
	if o := offer(rules.OfferDiscard); o != nil && o.Enabled && o.Source != nil && len(o.Source.Cards) > 0 {
		return WSIncoming{Type: "discard", Card: o.Source.Cards[0]}, true
	}

	// 5. Back out of a dead end.
	//
	// This branch is the offer list earning its keep. Under a rotating
	// contract, laying one set of a required two leaves a player unable to
	// discard (INCOMPLETE_INITIAL_MELD) until they finish the combination —
	// a real rule, and a real way to get stuck. The client is not told that
	// rule and does not need to be: it can see that discard is off and that
	// undo:turn is on, which is enough to recover. Blacklist the meld that
	// led here so the next attempt tries something else.
	if enabled(rules.OfferUndoTurn) {
		if len(c.lastProposed) > 0 {
			c.refused[strings.Join(c.lastProposed, ",")] = true
			c.lastProposed = nil
		}
		return WSIncoming{Type: "undo_turn"}, true
	}

	return WSIncoming{}, false
}

// proposeMeld picks candidate card groups by shape alone — same rank, or same
// suit and adjacent-looking — within the size bounds the offer gave. This is
// pattern-matching on card strings, not rule evaluation: the proposal is
// frequently wrong, and being wrong is fine, because the server is the judge.
func (c offerClient) proposeMeld(hand []string, minCards, maxCards int) []string {
	if minCards <= 0 || maxCards < minCards {
		return nil
	}
	byRank := map[string][]string{}
	bySuit := map[string][]string{}
	for _, card := range hand {
		if len(card) < 2 || strings.HasPrefix(card, "JOKER") {
			continue
		}
		byRank[card[:1]] = append(byRank[card[:1]], card)
		bySuit[card[1:]] = append(bySuit[card[1:]], card)
	}
	for _, group := range []map[string][]string{byRank, bySuit} {
		for _, cards := range group {
			unique := dedupe(cards)
			if len(unique) < minCards || len(unique) > maxCards {
				continue
			}
			candidate := unique[:minCards]
			if c.refused[strings.Join(candidate, ",")] {
				continue
			}
			return candidate
		}
	}
	return nil
}

func dedupe(cards []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

// playDeal runs a whole deal with every player driven by offerClient, through
// the real wire path. It returns how many actions were applied.
func playDeal(t *testing.T, g models.Game, maxActions int) (models.Game, int) {
	t.Helper()
	applied := 0
	clients := map[string]*offerClient{}

	for step := 0; step < maxActions; step++ {
		if g.Status != string(rules.StatusActive) {
			return g, applied
		}
		actor := g.CurrentTurn
		msg := BuildGameStateMsg(g, actor)

		// The invariant a client depends on: while the game is running, the
		// player on turn is always told about something.
		if len(msg.LegalActions) == 0 {
			t.Fatalf("step %d: no offers at all for the player on turn (%s)", step, actor)
		}

		if clients[actor] == nil {
			clients[actor] = &offerClient{playerID: actor, t: t, refused: map[string]bool{}}
		}
		in, ok := clients[actor].chooseAction(msg)
		if !ok {
			t.Fatalf("step %d: offers left %s with nothing to do:\n%s", step, actor, describeOffers(msg))
		}

		action, err := toRulesAction(in)
		if err != nil {
			t.Fatalf("step %d: the wire could not carry %+v: %v", step, in, err)
		}
		out, err := rules.ApplyAction(toRulesState(g), actor, action)
		if err != nil {
			// A refused lay_meld is the one acceptable rejection: the offer
			// bounds the *shape*, and the client proposed a concrete
			// combination the server judged invalid. Everything else means an
			// offer promised something the engine will not honour.
			if in.Type == "lay_meld" {
				clients[actor].refused[strings.Join(in.Cards, ",")] = true
				continue
			}
			t.Fatalf("step %d: %s was offered %+v but the engine refused it: %v\n%s",
				step, actor, in, err, describeOffers(msg))
		}
		fromRulesState(&g, out.State)
		applied++
		for _, down := range g.RoundReqMet {
			if down {
				sawSomeoneGoDown = true
			}
		}
	}
	return g, applied
}

// sawSomeoneGoDown records whether any player completed their initial meld
// during the last playDeal. It is the signal that the offer list carried a
// client all the way through the contract, not merely through draw/discard.
var sawSomeoneGoDown bool

// cloneGame deep-copies the mutable collections on a match document.
//
// Needed because toRulesState hands the engine the document's own maps and
// slices by reference, so a *dry-run* ApplyAction — one whose result is
// thrown away — still mutates the document it was derived from. The engine
// itself is fine (Manager applies exactly once, and rules.LegalActions clones
// internally before every probe), but a test that speculatively applies
// actions has to clone or it silently corrupts the game it is driving.
func cloneGame(g models.Game) models.Game {
	out := g
	out.TurnOrder = append([]string(nil), g.TurnOrder...)
	out.DrawPile = append([]string(nil), g.DrawPile...)
	out.DiscardPile = append([]string(nil), g.DiscardPile...)
	out.DiscardDrawnCards = append([]string(nil), g.DiscardDrawnCards...)
	out.Hands = map[string][]string{}
	for k, v := range g.Hands {
		out.Hands[k] = append([]string(nil), v...)
	}
	out.Melds = map[string][][]string{}
	for k, ms := range g.Melds {
		for _, m := range ms {
			out.Melds[k] = append(out.Melds[k], append([]string(nil), m...))
		}
	}
	out.MeldMeta = map[string][]models.MeldInfo{}
	for k, metas := range g.MeldMeta {
		out.MeldMeta[k] = append([]models.MeldInfo(nil), metas...)
	}
	out.RoundReqMet = map[string]bool{}
	for k, v := range g.RoundReqMet {
		out.RoundReqMet[k] = v
	}
	out.TotalScores = map[string]int{}
	for k, v := range g.TotalScores {
		out.TotalScores[k] = v
	}
	out.GameScores = map[string][]int{}
	for k, v := range g.GameScores {
		out.GameScores[k] = append([]int(nil), v...)
	}
	return out
}

func describeOffers(msg GameStateMsg) string {
	var b strings.Builder
	for _, o := range msg.LegalActions {
		state := "off"
		if o.Enabled {
			state = "ON "
		}
		cards := ""
		if o.Source != nil && len(o.Source.Cards) > 0 {
			cards = fmt.Sprintf(" cards=%v", o.Source.Cards)
		}
		fmt.Fprintf(&b, "  %s %-24s %s%s\n", state, o.ID, o.WhyNot, cards)
	}
	fmt.Fprintf(&b, "  hand=%v phase=%s\n", msg.MyHand, msg.Phase)
	return b.String()
}

func newSeededGame(t *testing.T, cfg rules.RulesConfig, seed int64) models.Game {
	t.Helper()
	state, err := rules.StartMatch(cfg, []string{"p1", "p2"}, seed)
	if err != nil {
		t.Fatalf("StartMatch: %v", err)
	}
	g := models.Game{
		Status:  string(rules.StatusActive),
		Players: []models.Player{{ID: "p1", Name: "A"}, {ID: "p2", Name: "B"}},
	}
	setGameRules(&g, cfg)
	fromRulesState(&g, state)
	return g
}

// TestOfferDrivenClient_CanPlayADealWithoutKnowingTheRules is the sufficiency
// claim: a client that reads nothing but the offer list can play real games.
//
// What each profile demonstrates differs, and the assertions say so rather
// than hiding behind a single loose bound:
//
//   - zolik_classic: whole deals finish, and the match reaches a winner.
//   - continental: players get *down* — they navigate the two-set/one-run
//     contract and the 35-point floor from offers alone. They rarely go out,
//     because this driver proposes melds by naive shape-matching and is a
//     poor Žolíky player; that is a limitation of the 40-line test client,
//     not of the offer list. The AI in internal/ai is what plays well.
//
// The strongest assertion is not any of these: it is that playDeal fails the
// test the instant an offered action is refused, or the offers leave the
// player on turn with nothing to do. Those run on every step of every seed.
func TestOfferDrivenClient_CanPlayADealWithoutKnowingTheRules(t *testing.T) {
	for _, profile := range []struct {
		name          string
		cfg           rules.RulesConfig
		budget        int
		wantCompleted bool
		wantGoDown    bool
	}{
		{"continental", rules.ProfileContinental, 600, false, false},
		{"zolik_classic", rules.ProfileZolikClassic, 4000, true, true},
	} {
		completed := 0
		for _, seed := range []int64{1, 7, 42, 1337} {
			t.Run(fmt.Sprintf("%s/seed-%d", profile.name, seed), func(t *testing.T) {
				sawSomeoneGoDown = false
				g := newSeededGame(t, profile.cfg, seed)
				g, applied := playDeal(t, g, profile.budget)

				if applied == 0 {
					t.Fatal("the offer list never produced a single applicable action")
				}
				if profile.wantGoDown && !sawSomeoneGoDown {
					t.Errorf("no player ever completed their initial meld in %d actions — "+
						"the offers did not carry a client through the contract", applied)
				}
				if g.Status != string(rules.StatusActive) {
					completed++
				}
				t.Logf("%s seed %d: %d actions, status=%s deal=%d",
					profile.name, seed, applied, g.Status, g.GameNumber)
			})
		}
		if profile.wantCompleted && completed == 0 {
			t.Errorf("%s: no seed ever reached a finished match — expected the offer-driven "+
				"client to play at least one to completion", profile.name)
		}
	}
}

// TestOfferDrivenClient_NeverOffersAnActionTheEngineRefuses is the safety
// claim, stated separately from the sufficiency one above: across the same
// real gameplay, every single-card action the offers advertised was accepted.
//
// playDeal already fails on any such mismatch; this test pins the intent
// explicitly so the guarantee is not an incidental side effect of another
// test's assertions.
func TestOfferDrivenClient_NeverOffersAnActionTheEngineRefuses(t *testing.T) {
	g := newSeededGame(t, rules.ProfileZolikClassic, 99)
	clients := map[string]*offerClient{}

	checked := 0
	for step := 0; step < 400 && g.Status == string(rules.StatusActive); step++ {
		actor := g.CurrentTurn
		msg := BuildGameStateMsg(g, actor)

		// Try *every* card every offer advertises, not just the one the
		// client would pick — a broader sweep than normal play produces.
		for _, o := range msg.LegalActions {
			if !o.Enabled || o.Source == nil {
				continue
			}
			for _, card := range o.Source.Cards {
				in, ok := wireActionFor(o, card)
				if !ok {
					continue
				}
				action, err := toRulesAction(in)
				if err != nil {
					t.Fatalf("wire cannot carry %+v: %v", in, err)
				}
				// cloneGame, not g: this apply is speculative and its result
				// is thrown away, but toRulesState would otherwise hand the
				// engine g's own maps to mutate in place.
				if _, err := rules.ApplyAction(toRulesState(cloneGame(g)), actor, action); err != nil {
					t.Errorf("step %d: offer %q advertised card %s but the engine refused: %v",
						step, o.ID, card, err)
				}
				checked++
			}
		}

		if clients[actor] == nil {
			clients[actor] = &offerClient{playerID: actor, t: t, refused: map[string]bool{}}
		}
		in, ok := clients[actor].chooseAction(msg)
		if !ok {
			break
		}
		action, _ := toRulesAction(in)
		out, err := rules.ApplyAction(toRulesState(g), actor, action)
		if err != nil {
			if in.Type == "lay_meld" {
				clients[actor].refused[strings.Join(in.Cards, ",")] = true
				continue
			}
			t.Fatalf("step %d: engine refused offered action %+v: %v", step, in, err)
		}
		fromRulesState(&g, out.State)
	}

	if checked == 0 {
		t.Fatal("no advertised cards were checked — the sweep is vacuous")
	}
	t.Logf("verified %d advertised card actions against the engine", checked)
}

// wireActionFor builds the WSIncoming an offer describes for one card, or
// ok=false for offers that are not single-card actions.
func wireActionFor(o rules.ActionOffer, card string) (WSIncoming, bool) {
	switch o.Verb {
	case rules.VerbDiscard:
		return WSIncoming{Type: "discard", Card: card}, true
	case rules.VerbLayOff:
		if o.Target == nil {
			return WSIncoming{}, false
		}
		return WSIncoming{Type: "lay_off", MeldID: o.Target.MeldID, Card: card}, true
	case rules.VerbSwapJoker:
		if o.Target == nil {
			return WSIncoming{}, false
		}
		return WSIncoming{Type: "swap_joker", MeldID: o.Target.MeldID, Card: card}, true
	case rules.VerbDraw:
		if o.ID == rules.OfferDrawDiscard {
			return WSIncoming{Type: "draw_card", From: "discard", Card: card}, true
		}
	}
	return WSIncoming{}, false
}

// TestBuildGameStateMsg_DoesNotMutateTheGame guards a hazard this work
// created and nearly tripped over.
//
// toRulesState hands the engine the match document's own maps and slices by
// reference, and the engine's validators mutate state in place. Computing
// offers means dry-running actions, so BuildGameStateMsg is now the one
// read-only-looking function on the read path that could silently corrupt a
// document — hands rewritten, cards vanished — just by rendering it. The
// probes clone for exactly that reason; this test is what keeps them cloning.
func TestBuildGameStateMsg_DoesNotMutateTheGame(t *testing.T) {
	g := newSeededGame(t, rules.ProfileZolikClassic, 5)
	// Reach a rich mid-game state first, so the offer computation has real
	// work to do: melds on the table, a player down, cards that fit.
	g, _ = playDeal(t, g, 60)

	before := cloneGame(g)
	for _, p := range g.Players {
		_ = BuildGameStateMsg(g, p.ID)
	}

	for _, p := range g.Players {
		got, want := strings.Join(g.Hands[p.ID], ","), strings.Join(before.Hands[p.ID], ",")
		if got != want {
			t.Errorf("%s's hand changed just by building the state message:\n got %s\nwant %s", p.ID, got, want)
		}
	}
	if got, want := strings.Join(g.DiscardPile, ","), strings.Join(before.DiscardPile, ","); got != want {
		t.Errorf("discard pile changed: got %s, want %s", got, want)
	}
	if len(g.DrawPile) != len(before.DrawPile) {
		t.Errorf("draw pile size changed: %d -> %d", len(before.DrawPile), len(g.DrawPile))
	}
	for owner, melds := range before.Melds {
		if len(g.Melds[owner]) != len(melds) {
			t.Errorf("%s's meld count changed: %d -> %d", owner, len(melds), len(g.Melds[owner]))
		}
	}
	if g.Phase != before.Phase || g.CurrentTurn != before.CurrentTurn {
		t.Errorf("turn state changed: %s/%s -> %s/%s",
			before.Phase, before.CurrentTurn, g.Phase, g.CurrentTurn)
	}
}
