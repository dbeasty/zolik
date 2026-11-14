package blackpeter

import (
	"math/rand"
	"strconv"

	"zolik/server/internal/module"
)

// How a seat nobody is sitting at plays Black Peter.
//
// It takes a card from the left, pointing at a face-down position at random.
// That is all there is to do: every card offered is face down, pairs go down
// by themselves, and where a taken card lands in a hand is decided by the
// table, not the holder. So Easy, Medium and Hard play the same game — a
// "stronger" choice would have to read a card it cannot see.
//
// It reads only the size of the hand it takes from, which every seat sees.
type bot struct{}

var _ module.Bot = bot{}

func (bot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil || s.Status != "active" || s.Intermission.Open || s.Current != seat.PlayerID {
		return module.ChooseAction(offers, []string{module.VerbContinue})
	}
	from := s.source(seat.PlayerID)
	if from < 0 {
		return module.Action{}, false
	}
	n := len(s.Seats[from].Hand)
	r := rand.New(rand.NewSource(seat.Seed + int64(s.Round)*1009 + int64(s.Draws)*31))
	return module.Action{OfferID: OfferDraw, Verb: VerbDraw, Params: map[string]string{
		ParamPick: strconv.Itoa(r.Intn(n)),
	}}, true
}
