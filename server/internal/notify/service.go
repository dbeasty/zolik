package notify

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/stats"
	"zolik/server/internal/ws"
)

// RoomID is the socket room every client's personal connection is held in,
// keyed by subject key. One room for everybody, like the waiting room, so the
// hub needs no new idea: a message to "user:abc" is a message to player
// "user:abc" in this room, on whichever instance holds the socket.
const RoomID = "__me__"

// The limits on announcing. Generous for a person opening a few tables in an
// evening and hard to turn into a way of pestering somebody.
const (
	hostTablesPerHour = 6
	perPairCooldown   = 10 * time.Minute
	// announcementTTL is how long the server remembers who it told about a
	// table, for "told once" and for revoking. Longer than any lobby lasts in
	// practice; the lobby's own retention is a day.
	announcementTTL = 6 * time.Hour
	// suggestionMatches bounds how far back "played recently" looks.
	suggestionMatches = 200
)

// History is the slice of the statistics store this package reads: who a
// subject has finished matches with.
type History interface {
	ListMatchesForSubject(ctx context.Context, key string, before time.Time, limit int) ([]stats.MatchResult, error)
}

// Accounts finds registered players by name, for adding by username.
type Accounts interface {
	FindByUsername(ctx context.Context, username string) (models.User, error)
	FindByID(ctx context.Context, id bson.ObjectID) (models.User, error)
}

// Tables reads a table by id or join code. Satisfied by match.Repository.
type Tables interface {
	Resolve(ctx context.Context, idOrCode string) (models.Match, error)
}

// Publisher delivers socket messages across instances. Satisfied by *ws.Hub.
type Publisher interface {
	Publish(room string, messages []ws.PlayerMessage)
}

// errBadRequest is a malformed request — a client bug, answered with a bare
// 400 rather than a worded refusal, because no player can produce it.
var errBadRequest = errors.New("notify: bad request")

// Service is the whole of the package's behaviour; handlers only translate.
type Service struct {
	repo     Repository
	history  History
	accounts Accounts
	tables   Tables
	hub      Publisher
	senders  Senders

	publicBaseURL  string
	vapidPublicKey string
	expoEnabled    bool
	gameLabel      func(string) string

	now func() time.Time

	mu        sync.Mutex
	announced map[string]*announcement // by match id
	hostLog   map[string][]time.Time   // host key → when each table was announced
	pairLast  map[string]time.Time     // "host>recipient" → last invite sent
}

// announcement is what the server remembers about one table it told people
// about. In memory rather than stored: losing it to a restart costs, at
// worst, one repeated invite and a revocation that never comes — the client
// expires stale invites on its own, and joining one that has started answers
// MATCH_ALREADY_STARTED as it always has.
type announcement struct {
	host   string
	invite Invite
	told   map[string]bool
	at     time.Time
}

// Invite is the wire shape of one table somebody was told about.
type Invite struct {
	ID       string `json:"id"`
	MatchID  string `json:"matchId"`
	JoinCode string `json:"joinCode"`
	ModuleID string `json:"moduleId"`
	// ModuleLabel is the game's own name, for a client with no words for
	// ModuleID — most game names are the same in every language and unkeyed.
	ModuleLabel string     `json:"moduleLabel,omitempty"`
	Variation   string     `json:"variation,omitempty"`
	Host        InviteHost `json:"host"`
	SentAt      time.Time  `json:"sentAt"`
}

type InviteHost struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

// Config is what a Service needs besides its stores.
type Config struct {
	PublicBaseURL  string
	VAPIDPublicKey string
	ExpoEnabled    bool
	// GameLabel is a module's own name for itself, used when no locale has
	// worded it — which, since game names are mostly the same in every
	// language, is the usual case.
	GameLabel func(moduleID string) string
}

func NewService(repo Repository, history History, accounts Accounts, tables Tables, hub Publisher, senders Senders, cfg Config) *Service {
	return &Service{
		repo: repo, history: history, accounts: accounts, tables: tables, hub: hub, senders: senders,
		publicBaseURL:  strings.TrimRight(cfg.PublicBaseURL, "/"),
		vapidPublicKey: cfg.VAPIDPublicKey,
		expoEnabled:    cfg.ExpoEnabled,
		gameLabel:      cfg.GameLabel,
		now:            func() time.Time { return time.Now().UTC() },
		announced:      map[string]*announcement{},
		hostLog:        map[string][]time.Time{},
		pairLast:       map[string]time.Time{},
	}
}

// ---- profile -------------------------------------------------------------

// ProfileView is a profile as its owner sees it.
type ProfileView struct {
	Profile
	FriendURL string `json:"friendUrl,omitempty"`
}

func (s *Service) view(p Profile) ProfileView {
	v := ProfileView{Profile: p}
	if s.publicBaseURL != "" {
		v.FriendURL = s.publicBaseURL + "/add/" + p.FriendCode
	}
	return v
}

// ensureProfile loads the caller's profile, creating it on first contact and
// refreshing the display name they are known by.
func (s *Service) ensureProfile(ctx context.Context, key, name string) (Profile, error) {
	p, err := s.repo.GetProfile(ctx, key)
	switch {
	case err == nil:
		if name != "" && p.Name != name {
			p.Name = name
			p.UpdatedAt = s.now()
			if err := s.repo.PutProfile(ctx, p); err != nil {
				return Profile{}, err
			}
		}
		if p.FriendCode != "" {
			return p, nil
		}
	case errors.Is(err, ErrNotFound):
		p = Profile{Key: key, Invites: InvitesCircle, Nearby: true, Name: name}
	default:
		return Profile{}, err
	}
	code, err := s.freshFriendCode(ctx)
	if err != nil {
		return Profile{}, err
	}
	p.FriendCode = code
	p.UpdatedAt = s.now()
	return p, s.repo.PutProfile(ctx, p)
}

// friendCodeAlphabet is the join-code alphabet: no 0/O or 1/I to misread.
const friendCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func (s *Service) freshFriendCode(ctx context.Context) (string, error) {
	for range 8 {
		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for i := range buf {
			buf[i] = friendCodeAlphabet[int(buf[i])%len(friendCodeAlphabet)]
		}
		code := string(buf)
		if _, err := s.repo.GetProfileByFriendCode(ctx, code); errors.Is(err, ErrNotFound) {
			return code, nil
		}
	}
	return "", errors.New("notify: could not find an unused friend code")
}

func (s *Service) Profile(ctx context.Context, uc auth.UserContext) (ProfileView, error) {
	p, err := s.ensureProfile(ctx, KeyFor(uc), uc.Username)
	return s.view(p), err
}

func (s *Service) UpdateProfile(ctx context.Context, uc auth.UserContext, invites *Invites, nearby *bool) (ProfileView, error) {
	p, err := s.ensureProfile(ctx, KeyFor(uc), uc.Username)
	if err != nil {
		return ProfileView{}, err
	}
	if invites != nil {
		switch *invites {
		case InvitesCircle, InvitesOff:
			p.Invites = *invites
		default:
			return ProfileView{}, errBadRequest
		}
	}
	if nearby != nil {
		p.Nearby = *nearby
	}
	p.UpdatedAt = s.now()
	return s.view(p), s.repo.PutProfile(ctx, p)
}

// ---- circle --------------------------------------------------------------

// Entry is one person on the circle screen.
type Entry struct {
	Key    string     `json:"key"`
	Name   string     `json:"name"`
	Avatar string     `json:"avatar,omitempty"`
	Status EdgeStatus `json:"status"`
	Since  time.Time  `json:"since"`
	Muted  *bool      `json:"muted,omitempty"`
}

// CircleView is everything the circle screen shows.
type CircleView struct {
	// Members are the people my tables reach (or will, once they accept).
	Members []Entry `json:"members"`
	// Requests are people asking for their tables to reach me.
	Requests []Entry `json:"requests"`
	// Notifiers are the people whose tables reach me, each mutable.
	Notifiers []Entry `json:"notifiers"`
}

func (s *Service) Circle(ctx context.Context, uc auth.UserContext) (CircleView, error) {
	me := KeyFor(uc)
	owned, err := s.repo.EdgesByOwner(ctx, me)
	if err != nil {
		return CircleView{}, err
	}
	held, err := s.repo.EdgesByMember(ctx, me)
	if err != nil {
		return CircleView{}, err
	}
	out := CircleView{Members: []Entry{}, Requests: []Entry{}, Notifiers: []Entry{}}
	for _, e := range owned {
		if e.Status == EdgeActive || e.Status == EdgePending {
			out.Members = append(out.Members, s.entry(ctx, e.MemberKey, e.MemberName, e))
		}
	}
	for _, e := range held {
		switch e.Status {
		case EdgePending:
			out.Requests = append(out.Requests, s.entry(ctx, e.OwnerKey, e.OwnerName, e))
		case EdgeActive:
			en := s.entry(ctx, e.OwnerKey, e.OwnerName, e)
			muted := e.Muted
			en.Muted = &muted
			out.Notifiers = append(out.Notifiers, en)
		}
	}
	for _, list := range [][]Entry{out.Members, out.Requests, out.Notifiers} {
		sort.SliceStable(list, func(i, j int) bool {
			return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
		})
	}
	return out, nil
}

// entry resolves the display details for key: the account if it is one, the
// profile they last called in with, or the name they had when the edge was
// made — in that order of freshness.
func (s *Service) entry(ctx context.Context, key, fallbackName string, e Edge) Entry {
	name, avatar := s.displayFor(ctx, key, fallbackName)
	return Entry{Key: key, Name: name, Avatar: avatar, Status: e.Status, Since: e.CreatedAt}
}

func (s *Service) displayFor(ctx context.Context, key, fallback string) (name, avatar string) {
	name = fallback
	if p, err := s.repo.GetProfile(ctx, key); err == nil {
		if p.Name != "" {
			name = p.Name
		}
		avatar = p.Avatar
	}
	if id, ok := strings.CutPrefix(key, "user:"); ok && s.accounts != nil {
		if oid, err := bson.ObjectIDFromHex(id); err == nil {
			if u, err := s.accounts.FindByID(ctx, oid); err == nil {
				name = u.Username
				if u.Preferences.Avatar != "" {
					avatar = u.Preferences.Avatar
				}
			}
		}
	}
	return name, avatar
}

// Suggestion is somebody the caller has finished a match with.
type Suggestion struct {
	Key          string    `json:"key"`
	Name         string    `json:"name"`
	Avatar       string    `json:"avatar,omitempty"`
	LastPlayedAt time.Time `json:"lastPlayedAt"`
	Matches      int       `json:"matches"`
}

// Suggestions lists the humans the caller has played, most recent first,
// leaving out anybody already in their circle.
func (s *Service) Suggestions(ctx context.Context, uc auth.UserContext) ([]Suggestion, error) {
	me := KeyFor(uc)
	played, err := s.played(ctx, me)
	if err != nil {
		return nil, err
	}
	owned, err := s.repo.EdgesByOwner(ctx, me)
	if err != nil {
		return nil, err
	}
	for _, e := range owned {
		if e.Status == EdgeActive || e.Status == EdgePending {
			delete(played, e.MemberKey)
		}
	}
	out := make([]Suggestion, 0, len(played))
	for _, sg := range played {
		if _, avatar := s.displayFor(ctx, sg.Key, sg.Name); avatar != "" {
			sg.Avatar = avatar
		}
		out = append(out, *sg)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastPlayedAt.After(out[j].LastPlayedAt) })
	return out, nil
}

// played is every human the subject has a finished match with, keyed by
// subject key. It is also the test for adding somebody without their say-so:
// sharing a finished table is the relationship that entitles you to ask.
func (s *Service) played(ctx context.Context, me string) (map[string]*Suggestion, error) {
	out := map[string]*Suggestion{}
	if s.history == nil {
		return out, nil
	}
	matches, err := s.history.ListMatchesForSubject(ctx, me, time.Time{}, suggestionMatches)
	if err != nil {
		return nil, err
	}
	for _, m := range matches {
		for _, p := range m.Participants {
			if p.Subject.Kind == stats.SubjectAI {
				continue
			}
			key := p.Subject.Key()
			if key == "" || key == me || !validKey(key) {
				continue
			}
			sg, ok := out[key]
			if !ok {
				name := p.Subject.Name
				if name == "" {
					name = p.Name
				}
				sg = &Suggestion{Key: key, Name: name}
				out[key] = sg
			}
			sg.Matches++
			if m.CompletedAt.After(sg.LastPlayedAt) {
				sg.LastPlayedAt = m.CompletedAt
			}
		}
	}
	return out, nil
}

// Add puts somebody in the caller's circle — directly when they have played
// together, as a request when found by username.
func (s *Service) Add(ctx context.Context, uc auth.UserContext, key, username string) (Entry, error) {
	me := KeyFor(uc)
	if _, err := s.ensureProfile(ctx, me, uc.Username); err != nil {
		return Entry{}, err
	}
	now := s.now()
	switch {
	case key != "":
		if !validKey(key) {
			return Entry{}, module.Error{Code: "UNKNOWN_PLAYER", Message: key}
		}
		if key == me {
			return Entry{}, module.Error{Code: "CANNOT_ADD_SELF"}
		}
		played, err := s.played(ctx, me)
		if err != nil {
			return Entry{}, err
		}
		sg, ok := played[key]
		if !ok {
			return Entry{}, module.Error{Code: "NOT_PLAYED_TOGETHER", Message: key}
		}
		e, err := s.putEdge(ctx, me, uc.Username, key, sg.Name, EdgeActive, SourcePlayed, now)
		if err != nil {
			return Entry{}, err
		}
		return s.entry(ctx, key, sg.Name, e), nil
	case username != "":
		if s.accounts == nil {
			return Entry{}, module.Error{Code: "UNKNOWN_PLAYER", Message: username}
		}
		u, err := s.accounts.FindByUsername(ctx, strings.TrimSpace(username))
		if err != nil {
			return Entry{}, module.Error{Code: "UNKNOWN_PLAYER", Message: username}
		}
		target := "user:" + u.ID.Hex()
		if target == me {
			return Entry{}, module.Error{Code: "CANNOT_ADD_SELF"}
		}
		// Somebody already in the circle stays as they are: re-adding by
		// name must not demote an active edge back to a request.
		status := EdgePending
		if existing, err := s.repo.GetEdge(ctx, me, target); err == nil && existing.Status == EdgeActive {
			status = EdgeActive
		}
		e, err := s.putEdge(ctx, me, uc.Username, target, u.Username, status, SourceUsername, now)
		if err != nil {
			return Entry{}, err
		}
		if status == EdgePending {
			s.publish(target, map[string]any{"type": "circle_changed"})
		}
		return s.entry(ctx, target, u.Username, e), nil
	}
	return Entry{}, errBadRequest
}

// putEdge writes owner→member, keeping any mute the member already set.
func (s *Service) putEdge(ctx context.Context, owner, ownerName, member, memberName string, status EdgeStatus, src EdgeSource, now time.Time) (Edge, error) {
	e := Edge{OwnerKey: owner, MemberKey: member, Status: status, Source: src, OwnerName: ownerName, MemberName: memberName, CreatedAt: now}
	if existing, err := s.repo.GetEdge(ctx, owner, member); err == nil {
		e.Muted = existing.Muted
		if existing.Status == status {
			e.CreatedAt = existing.CreatedAt
		}
	}
	e.ID = edgeID(owner, member)
	return e, s.repo.PutEdge(ctx, e)
}

// Remove stops the caller's tables reaching key.
func (s *Service) Remove(ctx context.Context, uc auth.UserContext, key string) error {
	me := KeyFor(uc)
	e, err := s.repo.GetEdge(ctx, me, key)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if e.Muted {
		e.Status = EdgeRemoved
		return s.repo.PutEdge(ctx, e)
	}
	return s.repo.DeleteEdge(ctx, me, key)
}

// Mute silences (or restores) key's tables for the caller.
func (s *Service) Mute(ctx context.Context, uc auth.UserContext, key string, muted bool) error {
	me := KeyFor(uc)
	e, err := s.repo.GetEdge(ctx, key, me)
	if errors.Is(err, ErrNotFound) {
		if !muted {
			return nil
		}
		// Muting somebody who has not added you yet is still a decision
		// worth keeping: it stands if they add you later.
		e = Edge{OwnerKey: key, MemberKey: me, Status: EdgeRemoved, CreatedAt: s.now()}
	} else if err != nil {
		return err
	}
	e.Muted = muted
	if !muted && e.Status == EdgeRemoved {
		return s.repo.DeleteEdge(ctx, key, me)
	}
	return s.repo.PutEdge(ctx, e)
}

// Accept turns key's request into an active edge, and adds them back: saying
// yes to hearing about somebody's tables is, in practice, saying you play
// together.
func (s *Service) Accept(ctx context.Context, uc auth.UserContext, key string) (Entry, error) {
	me := KeyFor(uc)
	e, err := s.repo.GetEdge(ctx, key, me)
	if err != nil || e.Status != EdgePending {
		return Entry{}, module.Error{Code: "UNKNOWN_PLAYER", Message: key}
	}
	now := s.now()
	e.Status = EdgeActive
	if err := s.repo.PutEdge(ctx, e); err != nil {
		return Entry{}, err
	}
	back, err := s.putEdge(ctx, me, uc.Username, key, e.OwnerName, EdgeActive, e.Source, now)
	if err != nil {
		return Entry{}, err
	}
	s.publish(key, map[string]any{"type": "circle_changed"})
	return s.entry(ctx, key, e.OwnerName, back), nil
}

func (s *Service) Decline(ctx context.Context, uc auth.UserContext, key string) error {
	me := KeyFor(uc)
	e, err := s.repo.GetEdge(ctx, key, me)
	if err != nil || e.Status != EdgePending {
		return nil
	}
	return s.repo.DeleteEdge(ctx, key, me)
}

// FriendPreview names the person behind a friend code, for the landing page.
func (s *Service) FriendPreview(ctx context.Context, code string) (name, avatar string, err error) {
	p, err := s.repo.GetProfileByFriendCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return "", "", module.Error{Code: "UNKNOWN_FRIEND_CODE", Message: code}
	}
	name, avatar = s.displayFor(ctx, p.Key, p.Name)
	return name, avatar, nil
}

// AddByFriendCode links the caller and the code's owner in both directions.
// Following somebody's link is consent from both: they chose to share it, and
// the caller chose to open it.
func (s *Service) AddByFriendCode(ctx context.Context, uc auth.UserContext, code string) (Entry, error) {
	me := KeyFor(uc)
	p, err := s.repo.GetProfileByFriendCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return Entry{}, module.Error{Code: "UNKNOWN_FRIEND_CODE", Message: code}
	}
	if p.Key == me {
		return Entry{}, module.Error{Code: "CANNOT_ADD_SELF"}
	}
	if _, err := s.ensureProfile(ctx, me, uc.Username); err != nil {
		return Entry{}, err
	}
	now := s.now()
	theirName, _ := s.displayFor(ctx, p.Key, p.Name)
	e, err := s.putEdge(ctx, me, uc.Username, p.Key, theirName, EdgeActive, SourceLink, now)
	if err != nil {
		return Entry{}, err
	}
	if _, err := s.putEdge(ctx, p.Key, theirName, me, uc.Username, EdgeActive, SourceLink, now); err != nil {
		return Entry{}, err
	}
	s.publish(p.Key, map[string]any{"type": "circle_changed"})
	return s.entry(ctx, p.Key, theirName, e), nil
}

// ---- announcing ----------------------------------------------------------

// Announce tells the host's circle about a table. keys, when given, narrows
// it to those people; either way nobody is told about the same table twice.
func (s *Service) Announce(ctx context.Context, uc auth.UserContext, matchID string, keys []string) (int, bool, error) {
	me := KeyFor(uc)
	m, err := s.tables.Resolve(ctx, matchID)
	if err != nil {
		return 0, false, module.Error{Code: "MATCH_NOT_FOUND", Message: matchID}
	}
	if m.HostID != uc.UserID {
		return 0, false, module.Error{Code: "NOT_THE_HOST"}
	}
	if m.Status != "lobby" {
		return 0, false, module.Error{Code: "MATCH_ALREADY_STARTED"}
	}

	edges, err := s.repo.EdgesByOwner(ctx, me)
	if err != nil {
		return 0, false, err
	}
	var only map[string]bool
	if len(keys) > 0 {
		only = make(map[string]bool, len(keys))
		for _, k := range keys {
			only[k] = true
		}
	}
	seated := map[string]bool{}
	for _, p := range m.Players {
		if k := KeyForPlayer(p); k != "" {
			seated[k] = true
		}
	}
	var candidates []string
	for _, e := range edges {
		if e.Status != EdgeActive || e.Muted || seated[e.MemberKey] {
			continue
		}
		if only != nil && !only[e.MemberKey] {
			continue
		}
		if p, err := s.repo.GetProfile(ctx, e.MemberKey); err == nil && p.Invites == InvitesOff {
			continue
		}
		candidates = append(candidates, e.MemberKey)
	}

	hostName, hostAvatar := s.displayFor(ctx, me, uc.Username)
	now := s.now()

	s.mu.Lock()
	s.expireLocked(now)
	a, known := s.announced[m.ID.Hex()]
	if !known {
		if len(s.hostLog[me]) >= hostTablesPerHour {
			s.mu.Unlock()
			return 0, false, module.Error{Code: "RATE_LIMITED"}
		}
		a = &announcement{
			host: me,
			invite: Invite{
				ID: m.ID.Hex(), MatchID: m.ID.Hex(), JoinCode: m.JoinCode,
				ModuleID: m.ModuleID, ModuleLabel: s.gameName("en", m.ModuleID), Variation: m.Variation,
				Host:   InviteHost{Key: me, Name: hostName, Avatar: hostAvatar},
				SentAt: now,
			},
			told: map[string]bool{},
			at:   now,
		}
		s.announced[m.ID.Hex()] = a
		s.hostLog[me] = append(s.hostLog[me], now)
	}
	var recipients []string
	for _, k := range candidates {
		if a.told[k] {
			continue
		}
		pair := me + ">" + k
		if last, ok := s.pairLast[pair]; ok && now.Sub(last) < perPairCooldown {
			continue
		}
		a.told[k] = true
		s.pairLast[pair] = now
		recipients = append(recipients, k)
	}
	invite := a.invite
	s.mu.Unlock()

	for _, k := range recipients {
		s.publish(k, map[string]any{"type": "table_invite", "invite": invite})
	}
	if len(recipients) > 0 {
		go s.pushInvite(recipients, invite)
	}
	return len(recipients), known && len(recipients) == 0, nil
}

func (s *Service) expireLocked(now time.Time) {
	for id, a := range s.announced {
		if now.Sub(a.at) > announcementTTL {
			delete(s.announced, id)
		}
	}
	for host, times := range s.hostLog {
		kept := times[:0]
		for _, t := range times {
			if now.Sub(t) < time.Hour {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(s.hostLog, host)
		} else {
			s.hostLog[host] = kept
		}
	}
	for pair, t := range s.pairLast {
		if now.Sub(t) >= perPairCooldown {
			delete(s.pairLast, pair)
		}
	}
}

// LobbyClosed withdraws every invite to a table that has started, filled or
// gone. Satisfies match.LobbyObserver.
func (s *Service) LobbyClosed(matchID string) {
	s.mu.Lock()
	a, ok := s.announced[matchID]
	if ok {
		delete(s.announced, matchID)
	}
	s.mu.Unlock()
	if !ok || len(a.told) == 0 {
		return
	}
	keys := make([]string, 0, len(a.told))
	for k := range a.told {
		keys = append(keys, k)
		s.publish(k, map[string]any{"type": "invite_revoked", "id": matchID})
	}
	go s.pushRevoke(keys, matchID)
}

func (s *Service) publish(key string, payload any) {
	if s.hub == nil {
		return
	}
	s.hub.Publish(RoomID, []ws.PlayerMessage{{PlayerID: key, Payload: payload}})
}

func (s *Service) inviteURL(joinCode string) string {
	return s.publicBaseURL + "/join/" + joinCode
}

func (s *Service) pushInvite(keys []string, inv Invite) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, k := range keys {
		s.pushTo(ctx, k, func(d Device) *Push {
			params := map[string]string{"host": inv.Host.Name, "game": s.gameName(d.Locale, inv.ModuleID)}
			return &Push{
				Title: text(d.Locale, "notify.push.inviteTitle", params),
				Body:  text(d.Locale, "notify.push.inviteBody", params),
				URL:   s.inviteURL(inv.JoinCode),
				Tag:   "invite:" + inv.MatchID,
				Data:  map[string]any{"type": "table_invite", "invite": inv, "url": "/join/" + inv.JoinCode},
			}
		})
	}
}

func (s *Service) pushRevoke(keys []string, matchID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, k := range keys {
		s.pushTo(ctx, k, func(d Device) *Push {
			// Browsers are not told: a web push must show a notification, and
			// Chrome puts up a generic "updated in the background" one for any
			// that does not — a worse outcome than a stale invite, which the
			// join screen already answers.
			if d.Kind == DeviceWebPush {
				return nil
			}
			return &Push{
				Tag:    "invite:" + matchID,
				Silent: true,
				Data:   map[string]any{"type": "invite_revoked", "id": matchID},
			}
		})
	}
}

func (s *Service) pushTo(ctx context.Context, key string, build func(Device) *Push) {
	devices, err := s.repo.DevicesFor(ctx, key)
	if err != nil {
		log.Printf("notify: devices for %s: %v", key, err)
		return
	}
	for _, d := range devices {
		p := build(d)
		if p == nil {
			continue
		}
		err := s.senders.Send(ctx, d, *p)
		switch {
		case errors.Is(err, ErrDeviceGone):
			_ = s.repo.DeleteDevice(ctx, d.ID)
		case err != nil:
			log.Printf("notify: push to %s device %s: %v", key, d.ID, err)
		}
	}
}

// ---- devices -------------------------------------------------------------

// DeviceRegistration is what a client sends to be reachable by push.
type DeviceRegistration struct {
	Kind         DeviceKind    `json:"kind"`
	Token        string        `json:"token,omitempty"`
	Subscription *Subscription `json:"subscription,omitempty"`
	Platform     string        `json:"platform,omitempty"`
	Locale       string        `json:"locale,omitempty"`
}

func (s *Service) RegisterDevice(ctx context.Context, uc auth.UserContext, reg DeviceRegistration) (string, error) {
	var ident string
	switch reg.Kind {
	case DeviceExpo:
		if reg.Token == "" || !s.expoEnabled {
			return "", module.Error{Code: "PUSH_UNAVAILABLE"}
		}
		ident = reg.Token
		reg.Subscription = nil
	case DeviceWebPush:
		if reg.Subscription == nil || reg.Subscription.Endpoint == "" || s.vapidPublicKey == "" {
			return "", module.Error{Code: "PUSH_UNAVAILABLE"}
		}
		ident = reg.Subscription.Endpoint
		reg.Token = ""
	default:
		return "", module.Error{Code: "PUSH_UNAVAILABLE"}
	}
	sum := sha256.Sum256([]byte(string(reg.Kind) + "|" + ident))
	d := Device{
		ID:           hex.EncodeToString(sum[:12]),
		SubjectKey:   KeyFor(uc),
		Kind:         reg.Kind,
		Token:        reg.Token,
		Subscription: reg.Subscription,
		Platform:     reg.Platform,
		Locale:       reg.Locale,
		LastSeenAt:   s.now(),
	}
	if _, err := s.ensureProfile(ctx, d.SubjectKey, uc.Username); err != nil {
		return "", err
	}
	return d.ID, s.repo.PutDevice(ctx, d)
}

// DeleteDevice unregisters one of the caller's own devices. Somebody else's
// id is answered exactly like an unknown one.
func (s *Service) DeleteDevice(ctx context.Context, uc auth.UserContext, id string) error {
	devices, err := s.repo.DevicesFor(ctx, KeyFor(uc))
	if err != nil {
		return err
	}
	for _, d := range devices {
		if d.ID == id {
			return s.repo.DeleteDevice(ctx, id)
		}
	}
	return nil
}

// PublicConfig is what a client needs to know before asking for permission.
type PublicConfig struct {
	VAPIDPublicKey *string `json:"vapidPublicKey"`
	Expo           bool    `json:"expo"`
}

func (s *Service) PublicConfig() PublicConfig {
	c := PublicConfig{Expo: s.expoEnabled}
	if s.vapidPublicKey != "" {
		k := s.vapidPublicKey
		c.VAPIDPublicKey = &k
	}
	return c
}

// Rekey carries a guest's circle onto the account it became.
func (s *Service) Rekey(ctx context.Context, guestID, userID string) error {
	return s.repo.Rekey(ctx, "guest:"+guestID, "user:"+userID)
}

// gameName is a module's name in locale, then its own label, then its id.
func (s *Service) gameName(locale, moduleID string) string {
	if name := gameName(locale, moduleID); name != moduleID {
		return name
	}
	if s.gameLabel != nil {
		if l := s.gameLabel(moduleID); l != "" {
			return l
		}
	}
	return moduleID
}

// SetGameLabel supplies the fallback game names once the module registry
// exists, which is after the service is built.
func (s *Service) SetGameLabel(f func(moduleID string) string) { s.gameLabel = f }
