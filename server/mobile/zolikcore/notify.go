package zolikcore

import (
	"strings"
	"sync"

	"zolik/server/internal/models"
)

// Notifier is the phone's own notification centre, implemented natively. A
// host has no internet to push through, so when somebody sits down at a table
// this phone hosts, the phone tells its owner itself: the app may be in the
// background with the screen off, and the one thing a host waiting for
// players wants is to hear that they arrived.
//
// The native side shows it only while the app is not in front; in front, the
// app's own banner says it.
type Notifier interface {
	Notify(tag, title, body, url string)
}

var (
	notifyMu    sync.Mutex
	notifier    Notifier
	joinedTitle = "{name} joined your table"
	joinedBody  = "{game} · tap to open the table"
)

// SetNotifier installs the phone's notification centre, or removes it with
// nil. It outlives hosts: set it once, and every table this process hosts
// uses it.
func SetNotifier(n Notifier) {
	notifyMu.Lock()
	defer notifyMu.Unlock()
	notifier = n
}

// SetJoinedTexts gives the notification its words in the player's language,
// with {name} and {game} where the player and the game go. The app passes its
// own translations, so the phone says it the way the app does.
func SetJoinedTexts(title, body string) {
	notifyMu.Lock()
	defer notifyMu.Unlock()
	if strings.TrimSpace(title) != "" {
		joinedTitle = title
	}
	if strings.TrimSpace(body) != "" {
		joinedBody = body
	}
}

// joinNotifier is the match runtime's join observer on a phone.
type joinNotifier struct {
	gameName func(moduleID string) string
}

func (j joinNotifier) PlayerJoined(m models.Match, p models.Player) {
	notifyMu.Lock()
	n, title, body := notifier, joinedTitle, joinedBody
	notifyMu.Unlock()
	if n == nil {
		return
	}
	game := m.ModuleID
	if j.gameName != nil {
		if g := j.gameName(m.ModuleID); g != "" {
			game = g
		}
	}
	fill := strings.NewReplacer("{name}", p.Name, "{game}", game)
	url := "/match/" + m.ID.Hex()
	if m.JoinCode != "" {
		url = "/join/" + m.JoinCode
	}
	// Off the runtime's goroutine: an observer must not block, and the
	// native side may take its time.
	go n.Notify("joined:"+m.ID.Hex()+":"+p.ID, fill.Replace(title), fill.Replace(body), url)
}
