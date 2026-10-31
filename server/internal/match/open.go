package match

import (
	"context"
	"time"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// listingTTL is how long a listed lobby stays on its game's page after its
// host's lobby screen last renewed it. The screen renews every 30 seconds, so
// this is three missed renewals: long enough to ride out a slow network, short
// enough that a host who closed the app stops being offered to strangers
// within a couple of minutes.
const listingTTL = 90 * time.Second

// openTablesLimit caps one game's list. A page of tables to pick from, not an
// archive: more than this and the newest are the ones worth showing.
const openTablesLimit = 20

// OpenTable is one listed lobby as a stranger sees it: whose it is, how full,
// and the code that seats them. Nothing about the people already sitting
// there beyond how many, since they agreed to play, not to be listed.
type OpenTable struct {
	MatchID    string `json:"matchId"`
	JoinCode   string `json:"joinCode"`
	Variation  string `json:"variation,omitempty"`
	HostName   string `json:"hostName"`
	HostID     string `json:"hostId"`
	HostAvatar string `json:"hostAvatar,omitempty"`
	Seated     int    `json:"seated"`
	Seats      int    `json:"seats"`
}

// OpenTables lists one game's lobbies that anybody may join: listed by their
// host, renewed within listingTTL, with a seat free, and not one viewerID is
// already sitting at (their own table is in their games, not here).
func (m *Manager) OpenTables(ctx context.Context, moduleID, viewerID string) ([]OpenTable, error) {
	mod := m.registry.Get(moduleID)
	if mod == nil {
		return nil, module.Error{Code: "UNKNOWN_MODULE", Message: moduleID}
	}
	found, err := m.repo.FindOpen(ctx, moduleID, time.Now().UTC().Add(-listingTTL), openTablesLimit)
	if err != nil {
		return nil, err
	}
	out := []OpenTable{}
	for _, match := range found {
		if match.Tutorial != nil || playerByID(match.Players, viewerID) != nil {
			continue
		}
		// The same count Join refuses on, so a table listed here is one the
		// viewer can actually sit down at.
		_, max := mod.Descriptor().SeatRange(match.Variation)
		seated := len(match.Players) + len(match.Reserved)
		if seated >= max {
			continue
		}
		host := playerByID(match.Players, match.HostID)
		if host == nil {
			continue
		}
		out = append(out, OpenTable{
			MatchID:    match.ID.Hex(),
			JoinCode:   match.JoinCode,
			Variation:  match.Variation,
			HostName:   host.Name,
			HostID:     host.ID,
			HostAvatar: host.Avatar,
			Seated:     seated,
			Seats:      max,
		})
	}
	return out, nil
}

// SetListed lists a lobby on its game's page, or takes it off. Called again
// with listed true it renews the listing: the write moves UpdatedAt, which is
// what OpenTables reads. Host only, and only before the deal.
func (m *Manager) SetListed(ctx context.Context, idOrCode, hostID string, listed bool) (models.Match, error) {
	e, err := m.lockMatch(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	defer e.mu.Unlock()
	match := e.match
	if match.HostID != hostID {
		return models.Match{}, module.Error{Code: "NOT_THE_HOST", Message: hostID}
	}
	if match.Status != "lobby" {
		return models.Match{}, module.Error{Code: "MATCH_ALREADY_STARTED"}
	}
	if match.Tutorial != nil {
		// A lesson is one person's walk-through, never a table to share.
		return match, nil
	}
	next := match
	next.Listed = listed
	if err := m.saveLocked(ctx, e, next); err != nil {
		return models.Match{}, err
	}
	return e.match, nil
}
