// Package botsettings persists the operator's runtime switches for bots:
// whether each game's AI seats play the trained model the binary ships
// (learn.HardModels), and how much the bot governor may do (internal/botgov).
//
// One document, "bots", in the settings collection (a namespace under KDB),
// holding a map keyed by game. The app reads it once at boot into the
// in-process switch, and the admin console writes it before flipping the
// switch, so what a restart comes back to is what the operator last chose.
// A game with no entry is off: deploying the feature changes nothing.
package botsettings

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DocID is the one document this package reads and writes.
const DocID = "bots"

// HardModel is one game's switch, and who last moved it.
type HardModel struct {
	Enabled   bool      `bson:"enabled" json:"enabled"`
	UpdatedBy string    `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	UpdatedAt time.Time `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

// Governor is the bot governor's mode as the operator last set it — "off",
// "observe" or "enforce" — and who set it. A zero Mode has never been set,
// and the server's BOT_GOVERNOR setting stands.
type Governor struct {
	Mode      string    `bson:"mode,omitempty" json:"mode,omitempty"`
	UpdatedBy string    `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	UpdatedAt time.Time `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

// Doc is the stored document.
type Doc struct {
	ID        string               `bson:"_id" json:"_id"`
	HardModel map[string]HardModel `bson:"hardModel" json:"hardModel"`
	Governor  Governor             `bson:"governor,omitempty" json:"governor,omitempty"`
}

// Store is the persistence behind the switch, on either engine.
type Store interface {
	// HardModels returns every game's stored setting. A game with no entry
	// has never been changed, and is off.
	HardModels(ctx context.Context) (map[string]HardModel, error)
	// SetHardModel records one game's setting, leaving the others as they are.
	SetHardModel(ctx context.Context, game string, s HardModel) error
	// Governor returns the stored governor mode; a zero Mode was never set.
	Governor(ctx context.Context) (Governor, error)
	// SetGovernor records the governor mode, leaving the games as they are.
	SetGovernor(ctx context.Context, g Governor) error
}

// checkGame refuses a key that would not survive as a Mongo field name. The
// console only ever sends registry names, which are plain words; this is the
// store being safe on its own terms.
func checkGame(game string) error {
	if game == "" || strings.ContainsAny(game, ".$\x00") {
		return fmt.Errorf("botsettings: bad game name %q", game)
	}
	return nil
}
