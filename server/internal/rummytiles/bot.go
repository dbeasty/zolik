package rummytiles

import (
	"sort"

	"zolik/server/internal/module"
)

// bot is Rummy Tiles' own player.
//
// module.OfferBot cannot play this game at all: the offers describe single
// moves and the game is about combinations, so a bot taking whichever offer
// comes first would shuffle the table forever without ever laying a set.
//
// This is the greedy policy from §3.6 of the plan: lay every set formable
// from the hand alone, then extend existing sets tile by tile, then draw. It
// composes its own moves directly from decoded state rather than through
// offers, since place/add/take are Composite — a person, or a bot, composes
// them, the offer list only says the move exists. Every skill plays this same
// policy for now; the solver §3.6 promises for hard/expert is B5, not yet
// built.
type bot struct{}

var _ module.Bot = bot{}

func (b bot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil {
		return module.Action{}, false
	}
	if s.Intermission.Open || s.Current != seat.PlayerID || s.Workspace == nil {
		return module.ChooseAction(offers, nil)
	}

	hand := s.Hands[seat.PlayerID]

	if combo, ok := findFormableSet(hand); ok {
		return module.Action{Verb: VerbPlace, Cards: combo}, true
	}

	if s.InitialMeld[seat.PlayerID] {
		for _, set := range s.Workspace.Sets {
			for _, tile := range sortedUnique(hand) {
				extended := append(append([]string(nil), set.Cards...), tile)
				if _, _, ok := validateSet(extended); ok {
					return module.Action{Verb: VerbAdd, Target: set.ID, Cards: []string{tile}}, true
				}
			}
		}
	}

	if o := findOffer(offers, OfferCommit); o != nil && o.Enabled {
		return module.Action{OfferID: OfferCommit, Verb: VerbCommit}, true
	}
	if o := findOffer(offers, OfferDraw); o != nil && o.Enabled {
		return module.Action{OfferID: OfferDraw, Verb: VerbDraw}, true
	}
	return module.ChooseAction(offers, nil)
}

// findFormableSet looks for one valid group or run using only real tiles
// already in hand — no jokers, which the greedy tier leaves for a player or a
// smarter bot to spend deliberately rather than on the first set that fits.
func findFormableSet(hand []string) ([]string, bool) {
	byNumber := map[int][]string{}
	for _, t := range hand {
		if isJoker(t) {
			continue
		}
		byNumber[numberOf(t)] = append(byNumber[numberOf(t)], t)
	}
	for n := 1; n <= 13; n++ {
		seenColour := map[string]bool{}
		var group []string
		for _, t := range byNumber[n] {
			c := colourOf(t)
			if seenColour[c] {
				continue
			}
			seenColour[c] = true
			group = append(group, t)
			if len(group) == 4 {
				break
			}
		}
		if len(group) >= 3 {
			return group, true
		}
	}

	for _, c := range colours {
		numToTile := map[int]string{}
		var nums []int
		for _, t := range hand {
			if isJoker(t) || colourOf(t) != c {
				continue
			}
			n := numberOf(t)
			if _, dup := numToTile[n]; dup {
				continue
			}
			numToTile[n] = t
			nums = append(nums, n)
		}
		sort.Ints(nums)
		i := 0
		for i < len(nums) {
			j := i
			for j+1 < len(nums) && nums[j+1] == nums[j]+1 {
				j++
			}
			if j-i+1 >= 3 {
				run := make([]string, 0, j-i+1)
				for k := i; k <= j; k++ {
					run = append(run, numToTile[nums[k]])
				}
				return run, true
			}
			i = j + 1
		}
	}
	return nil, false
}

func findOffer(offers []module.ActionOffer, id string) *module.ActionOffer {
	for i := range offers {
		if offers[i].ID == id {
			return &offers[i]
		}
	}
	return nil
}
