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

func (c *Client) CreateGame(initialMeldMin *int) (gameID, joinCode string, err error) {
	body := map[string]any{}
	if initialMeldMin != nil {
		body["initialMeldMinimum"] = *initialMeldMin
	}
	var resp struct {
		GameID   string `json:"gameId"`
		JoinCode string `json:"joinCode"`
	}
	if err := c.postJSON("/games", body, &resp, true); err != nil {
		return "", "", err
	}
	return resp.GameID, resp.JoinCode, nil
}

func (c *Client) JoinGame(idOrCode string) (string, error) {
	var resp struct {
		GameID string `json:"gameId"`
	}
	if err := c.postJSON("/games/"+url.PathEscape(idOrCode)+"/join", nil, &resp, true); err != nil {
		return "", err
	}
	return resp.GameID, nil
}

func (c *Client) AddAI(idOrCode, difficulty string) error {
	return c.postJSON("/games/"+url.PathEscape(idOrCode)+"/add-ai", map[string]string{
		"difficulty": difficulty,
	}, &struct{}{}, true)
}

func (c *Client) StartGame(idOrCode string) error {
	return c.postJSON("/games/"+url.PathEscape(idOrCode)+"/start", nil, &struct{}{}, true)
}

func (c *Client) GetLobby(idOrCode string) (LobbyGame, error) {
	var g LobbyGame
	if err := c.getJSON("/games/"+url.PathEscape(idOrCode), &g, false); err != nil {
		return LobbyGame{}, err
	}
	return g, nil
}

func (c *Client) DialWS(gameID string) (*websocket.Conn, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s/ws/games/%s?token=%s", scheme, u.Host, gameID, url.QueryEscape(c.Token))
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	return conn, err
}

func (c *Client) SendWS(conn *websocket.Conn, action WSAction) error {
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
