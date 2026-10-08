// Package marias implements Mariáš (volený and licitovaný, three players, or
// four with the dealer sitting out) as a game module.
//
// It is Czech pub Mariáš as the Český svaz mariáše writes it down for
// "bodovaný volený mariáš" (rules in force from 8.5.2007), with the
// simplifications docs/marias-rules.md lists and a lobby can see: an engine
// that refuses illegal cards has no use for renonc penalties. At a table of
// four the dealer pauzíruje — sits the deal out, and neither pays nor is
// paid (čtyřhranný mariáš).
//
// This file is step 0 of docs/marias-plan.md: the rules pinned as code — the
// options, the written rules and the payment table — before the engine that
// plays them. The Module type grows into a module.GameModule in step 3.
//
// Cards use the server's usual codes; the German suits map onto them as
// H červené, D kule, C žaludy, S zelené, and J is the spodek, Q the svršek.
package marias

import "zolik/server/internal/tricks"

// Module is Mariáš.
type Module struct{}

// Card order within a suit, highest first.
const (
	orderTrump tricks.Order = "ATKQJ987" // hra, sedma, sto: the ten sits under the ace
	orderPlain tricks.Order = "AKQJT987" // betl, durch: the ten back in its natural place
)

// suitRed is the suit whose trumps double every payment (červené platí
// dvojnásob).
const suitRed = 'H'

// Card points: every ace and ten, and the last trick.
const (
	pointsSharp     = 10 // eso, desítka
	pointsLastTrick = 10
	pointsTotal     = 90 // 8 sharp cards + the last trick

	marriagePlain = 20
	marriageTrump = 40

	hundred = 100 // sto: needs at least one marriage, since the cards make 90
)

// Option names, as the lobby sends them.
const (
	OptDeals          = "deals"
	OptTariff         = "tariff"
	OptRedDoubles     = "redDoubles"
	OptFlekLimit      = "flekLimit"
	OptZLidu          = "zLidu"
	OptShowCardPoints = "showCardPoints"
)

// Tariff choices.
const (
	TariffCSM = 0 // Český svaz mariáše: betl 15, durch 30
	TariffPub = 1 // the common pub table: betl 5, durch 10
)

// FlekUnlimited is the flekLimit value for "flek, re, tutti, … for as long as
// anyone dares".
const FlekUnlimited = 0

const (
	variationVoleny = "voleny"
	defaultDeals    = 12
)

// config is a match config resolved against the variation's defaults, so the
// rules, the deal and the settlement read one answer each.
type config struct {
	variation      string
	deals          int
	tariff         Tariff
	redDoubles     bool
	flekLimit      int // FlekUnlimited, or the most doublings any one part takes
	zLidu          bool
	showCardPoints bool
}
