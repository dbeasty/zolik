package match

import (
	"fmt"
	"time"

	"zolik/server/internal/models"
)

// The action log and checkpoint boards, as the repositories store them.
//
// A match used to carry its whole log inside its own document, rewritten on
// every move. Under a store that keeps every version of every document, that
// made a match's history cost the square of its length. In the pages layout
// (models.LogFormatPages) the log lives beside the match instead: a move
// rewrites the game state and one page of LogPageSize actions.
//
// The repositories own the layout. Callers see a match with ActionCount and
// a checkpoint index, and ask the repository for the log itself.

// newMatchLogFormat is the layout new matches are created in.
//
// Pages, now that the release before this one reads and writes them: rolling
// back to it is safe. Rolling back past it, to a build that has never heard of
// pages, is not.
var newMatchLogFormat = models.LogFormatPages

// migrateLegacyOnAppend moves an original-layout match to pages the next time
// it accepts a move. A finished match never moves, and keeps its inline log
// until retention removes it.
var migrateLegacyOnAppend = true

// settle turns a match as stored into the shape callers get: ActionCount
// filled in whichever layout it is in, and the original layout's inline log
// and boards dropped — the repository reads them again when asked.
func settle(m models.Match) models.Match {
	if m.LogFormat != models.LogFormatPages {
		m.ActionCount = len(m.LegacyActionLog)
	}
	m.LegacyActionLog = nil
	if len(m.Checkpoints) > 0 {
		cps := make([]models.MatchCheckpoint, len(m.Checkpoints))
		for i, c := range m.Checkpoints {
			if len(c.LegacyState) > 0 {
				c.Board = true
			}
			c.LegacyState = nil
			cps[i] = c
		}
		m.Checkpoints = cps
	}
	return m
}

// actionsIn is how many actions a stored version of a match had accepted,
// read from the version itself. Monotonic across every version of a match,
// including the one that moved it from the original layout to pages.
func actionsIn(m models.Match) int {
	if m.LogFormat == models.LogFormatPages {
		return m.ActionCount
	}
	return len(m.LegacyActionLog)
}

// logPage is LogPageSize consecutive actions of one match, the first being
// Seq Page*LogPageSize+1.
type logPage struct {
	Match   string      `bson:"m"`
	Page    int         `bson:"p"`
	Entries []pageEntry `bson:"e"`
}

type pageEntry struct {
	Seq    int            `bson:"s"`
	Player string         `bson:"p"`
	Action models.JSONDoc `bson:"a"`
	// At is Unix milliseconds: the precision the original layout's dates had.
	At int64 `bson:"t"`
}

// boardRecord is the board stored at one checkpoint.
type boardRecord struct {
	Match string         `bson:"m"`
	Seq   int            `bson:"s"`
	State models.JSONDoc `bson:"state"`
}

func pageOf(seq int) int { return (seq - 1) / models.LogPageSize }

func pageKey(matchHex string, page int) string { return fmt.Sprintf("%s/a/%d", matchHex, page) }

func boardKey(matchHex string, seq int) string { return fmt.Sprintf("%s/b/%d", matchHex, seq) }

func toPageEntry(a models.MatchAction) pageEntry {
	return pageEntry{Seq: a.Seq, Player: a.PlayerID, Action: a.Action, At: a.At.UnixMilli()}
}

func fromPageEntry(e pageEntry) models.MatchAction {
	return models.MatchAction{Seq: e.Seq, PlayerID: e.Player, Action: e.Action, At: time.UnixMilli(e.At).UTC()}
}

// appendToPage places entry in page at the slot its Seq names. Anything at or
// past that slot is dropped first: a page can hold an entry the match never
// recorded, left by an append whose match write did not land.
func appendToPage(p logPage, entry models.MatchAction) logPage {
	slot := (entry.Seq - 1) % models.LogPageSize
	if len(p.Entries) > slot {
		p.Entries = p.Entries[:slot]
	}
	p.Entries = append(p.Entries, toPageEntry(entry))
	return p
}

// pagesFor splits a whole log into pages, for moving an original-layout match
// to pages in one go.
func pagesFor(matchHex string, log []models.MatchAction) []logPage {
	var out []logPage
	for _, a := range log {
		n := pageOf(a.Seq)
		if len(out) == 0 || out[len(out)-1].Page != n {
			out = append(out, logPage{Match: matchHex, Page: n})
		}
		out[len(out)-1].Entries = append(out[len(out)-1].Entries, toPageEntry(a))
	}
	return out
}

// sliceLog returns actions from+1 … to of a whole log, stopping early if the
// log does not hold them contiguously.
func sliceLog(log []models.MatchAction, from, to int) []models.MatchAction {
	var out []models.MatchAction
	for seq := from + 1; seq <= to; seq++ {
		if seq-1 >= len(log) || log[seq-1].Seq != seq {
			break
		}
		out = append(out, log[seq-1])
	}
	return out
}
