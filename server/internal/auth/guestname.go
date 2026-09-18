package auth

import (
	"hash/fnv"
	"math/rand"
)

// The name a player is given when they have not said what to call themselves.
//
// Every guest used to be "Player" (the app's prefilled field) or "Guest" (the
// SSH host, and any client that sent no name at all), which at a table of four
// is not a name but a shrug: the seat tiles read Player, Player, Player, and
// the only way to tell whose turn it is is to count. A name has to *identify*
// before it does anything else.
//
// Numbering would identify — Guest 1, Guest 2 — and is rejected for the reason
// written over suggestUsername: a number is a queue ticket, and a queue ticket
// is a poor thing to be called for an evening of cards. So the default is a
// two-word name drawn from a roster, in the same spirit as the bot personas in
// module.Persona (Rookie Rita, Master Miroslav) but deliberately a different
// *shape* — those are adjective-plus-forename, these are adjective-plus-
// creature, so a guest is never mistaken for an opponent nobody is sitting at.
//
// The roster is English and untranslated, exactly as the personas and the
// avatar labels are. That is a considered position rather than an oversight:
// these are proper names, the thing a translation bundle is least able to
// help with, and the field is editable on the very screen that suggests one.
//
// A near-copy of this roster lives in the app, in
// client-react-native/src/lib/guestName.ts, so the name can be shown in the
// field before anything is sent. The two are allowed to drift. Nothing ever
// compares them — unlike the message keys in serverKeys.json, where drift
// reaches a player as SCREAMING_SNAKE — and a guest named from either list is
// equally well named.
//
// 64 x 64 is 4096 names, which puts two guests at a six-seat table sharing one
// at about one table in 270. That is rare enough to be a curiosity rather than
// a defect, and the remedy — typing over the suggestion — is in front of them.
var guestAdjectives = []string{
	"Amber", "Arctic", "Autumn", "Bold", "Brave", "Brisk", "Bright", "Calm",
	"Canny", "Cheery", "Clever", "Copper", "Coral", "Crimson", "Curious", "Daring",
	"Dusty", "Eager", "Emerald", "Fearless", "Fleet", "Frosty", "Gentle", "Golden",
	"Grand", "Happy", "Hasty", "Honest", "Humble", "Indigo", "Ivory", "Jolly",
	"Keen", "Kindly", "Lively", "Lucky", "Merry", "Mighty", "Nimble", "Noble",
	"Patient", "Plucky", "Polite", "Proud", "Quick", "Quiet", "Rapid", "Restless",
	"Royal", "Ruby", "Rusty", "Scarlet", "Sharp", "Silver", "Sly", "Snowy",
	"Solid", "Spry", "Steady", "Stormy", "Sunny", "Swift", "Velvet", "Witty",
}

var guestNouns = []string{
	"Otter", "Badger", "Falcon", "Heron", "Marten", "Lynx", "Sparrow", "Magpie",
	"Raven", "Robin", "Fox", "Hare", "Stag", "Ibex", "Bison", "Boar",
	"Wolf", "Owl", "Crane", "Swallow", "Finch", "Kestrel", "Osprey", "Puffin",
	"Curlew", "Beaver", "Weasel", "Ferret", "Mole", "Dormouse", "Squirrel", "Hedgehog",
	"Chamois", "Lark", "Wren", "Starling", "Kite", "Harrier", "Pike", "Perch",
	"Trout", "Salmon", "Newt", "Frog", "Turtle", "Bee", "Moth", "Cricket",
	"Beetle", "Firefly", "Dragonfly", "Comet", "Ember", "Lantern", "Compass", "Anchor",
	"Beacon", "Pebble", "Willow", "Aspen", "Birch", "Juniper", "Thistle", "Clover",
}

// GuestNameFor invents a display name, the same one every time for the same
// seed.
//
// Stability is the point of the seed, and the seed callers pass is the guest
// id — the device's durable identity. A guest who signs out and comes back, or
// who plays from a terminal that stores no name of its own, is greeted as the
// same person rather than as somebody new each time; and because the id is
// already unique per device, two devices agreeing on a name is a matter of
// chance rather than of everybody landing on the same default.
//
// An empty seed means there is nothing durable to hang a name on yet, so one
// is drawn at random. That is still 4096 ways to not be called Player.
func GuestNameFor(seed string) string {
	if seed == "" {
		return guestName(rand.Uint64())
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	return guestName(h.Sum64())
}

// guestName cuts one number into the two independent indices. Dividing out the
// first list's length before taking the second index keeps the halves from
// moving together, which is what a pair of plain moduli of the same number
// would do whenever the two lengths share a factor — and these two are equal.
func guestName(n uint64) string {
	adj := guestAdjectives[n%uint64(len(guestAdjectives))]
	noun := guestNouns[(n/uint64(len(guestAdjectives)))%uint64(len(guestNouns))]
	return adj + " " + noun
}
