package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	BaseURL string
	Token   string
	UserID  string

	client *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) SetAuth(accessToken, userID string) {
	c.Token = accessToken
	c.UserID = userID
}

func (c *Client) GuestLogin(name string) error {
	var resp struct {
		AccessToken string `json:"accessToken"`
		UserID      string `json:"userId"`
	}
	if err := c.postJSON("/auth/guest", map[string]string{"guestName": name}, &resp, false); err != nil {
		return err
	}
	c.Token = resp.AccessToken
	c.UserID = resp.UserID
	if c.UserID == "" {
		c.UserID = resp.AccessToken
	}
	return nil
}

func (c *Client) Login(username, password string) error {
	var resp struct {
		AccessToken string `json:"accessToken"`
	}
	if err := c.postJSON("/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, &resp, false); err != nil {
		return err
	}
	c.Token = resp.AccessToken
	return c.loadSubjectFromToken()
}

func (c *Client) loadSubjectFromToken() error {
	var me struct {
		ID string `json:"id"`
	}
	if err := c.getJSON("/users/me", &me, true); err == nil && me.ID != "" {
		c.UserID = me.ID
		return nil
	}
	c.UserID = c.Token
	return nil
}

// --- matches ---------------------------------------------------------------
//
// Six methods, none of which names a game. What they replaced —
// CreateGame/JoinGame/AddAI/StartGame/GetLobby/GetScoreboard plus a rummy
// action sender — was the same six ideas with rummy baked into each one, down
// to CreateGame taking an initial-meld minimum as its only argument.

// Modules lists every game this server hosts.
func (c *Client) Modules() ([]Module, error) {
	var resp struct {
		Modules []Module `json:"modules"`
	}
	if err := c.getJSON("/modules", &resp, false); err != nil {
		return nil, err
	}
	return resp.Modules, nil
}

func (c *Client) CreateMatch(moduleID, variation string, options map[string]int) (matchID, joinCode string, err error) {
	body := map[string]any{"moduleId": moduleID}
	if variation != "" {
		body["variation"] = variation
	}
	if len(options) > 0 {
		body["options"] = options
	}
	var resp struct {
		MatchID  string `json:"matchId"`
		JoinCode string `json:"joinCode"`
	}
	if err := c.postJSON("/matches", body, &resp, true); err != nil {
		return "", "", err
	}
	return resp.MatchID, resp.JoinCode, nil
}

// JoinMatch joins by match id or by the short code a host reads out.
func (c *Client) JoinMatch(idOrCode string) (string, error) {
	var resp struct {
		MatchID string `json:"matchId"`
	}
	if err := c.postJSON("/matches/"+url.PathEscape(idOrCode)+"/join", nil, &resp, true); err != nil {
		return "", err
	}
	return resp.MatchID, nil
}

// AddBot seats an opponent. The runtime drives it from the module's own offer
// list, so this works for a game nobody has written yet.
func (c *Client) AddBot(idOrCode string) error {
	return c.postJSON("/matches/"+url.PathEscape(idOrCode)+"/add-bot", nil, &struct{}{}, true)
}

func (c *Client) StartMatch(idOrCode string) error {
	return c.postJSON("/matches/"+url.PathEscape(idOrCode)+"/start", nil, &struct{}{}, true)
}

// GetMatch reads a viewer's state over plain HTTP; the socket is the live path.
func (c *Client) GetMatch(idOrCode string) (MatchState, error) {
	var m MatchState
	q := ""
	if c.UserID != "" {
		q = "?as=" + url.QueryEscape(c.UserID)
	}
	if err := c.getJSON("/matches/"+url.PathEscape(idOrCode)+q, &m, false); err != nil {
		return MatchState{}, err
	}
	return m, nil
}

func (c *Client) DialWS(matchID string) (*websocket.Conn, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s/ws/matches/%s?token=%s", scheme, u.Host, matchID, url.QueryEscape(c.Token))
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	return conn, err
}

func (c *Client) SendWS(conn *websocket.Conn, action Action) error {
	return conn.WriteJSON(action)
}

func (c *Client) CreateScoringSession(players []string) (string, error) {
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.postJSON("/scoring-sessions", map[string]any{"players": players}, &resp, false); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *Client) GetScoringSession(id string) (map[string]any, error) {
	var resp map[string]any
	if err := c.getJSON("/scoring-sessions/"+url.PathEscape(id), &resp, false); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) PatchScoringSession(id string, round int, scores map[string]int) error {
	return c.patchJSON("/scoring-sessions/"+url.PathEscape(id), map[string]any{
		"round":  round,
		"scores": scores,
	})
}

func (c *Client) ExportScoringSession(id string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/scoring-sessions/"+url.PathEscape(id)+"/export", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("export: %s", string(b))
	}
	return string(b), nil
}

func (c *Client) GetStats() (map[string]any, error) {
	var out map[string]any
	if err := c.getJSON("/users/me/stats", &out, true); err != nil {
		return nil, err
	}
	return out, nil
}

// GetScoreboard returns the standings of one match — running or finished. It
// needs no authentication and works on a join code as well as an ID, matching
// the lobby endpoint.
// GetHeadToHead returns this player's record against each opponent they have
// faced, bots included.
func (c *Client) GetHeadToHead() (map[string]any, error) {
	var out map[string]any
	if err := c.getJSON("/users/me/head-to-head", &out, true); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) postJSON(path string, body any, out any, auth bool) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth && c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.decode(resp, out)
}

func (c *Client) patchJSON(path string, body any) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPatch, c.BaseURL+path, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *Client) getJSON(path string, out any, auth bool) error {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	if auth && c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.decode(resp, out)
}

func (c *Client) decode(resp *http.Response, out any) error {
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s", strings.TrimSpace(string(b)))
	}
	if out == nil || len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, out)
}
