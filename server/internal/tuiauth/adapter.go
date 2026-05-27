package tuiauth

import (
	"context"

	"zolik/client-tui/ssh"
	"zolik/server/internal/auth"
)

// Adapter implements client-tui SSH authentication using server auth handlers.
type Adapter struct {
	Auth *auth.Handlers
}

func (a *Adapter) Guest(ctx context.Context, guestName string) (ssh.Session, error) {
	tok, err := a.Auth.GuestSession(ctx, guestName)
	if err != nil {
		return ssh.Session{}, err
	}
	return ssh.Session{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		UserID:       tok.UserID,
		Username:     tok.Username,
		IsGuest:      tok.IsGuest,
	}, nil
}

func (a *Adapter) Login(ctx context.Context, username, password string) (ssh.Session, error) {
	tok, err := a.Auth.LoginSession(ctx, username, password)
	if err != nil {
		return ssh.Session{}, err
	}
	return ssh.Session{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		UserID:       tok.UserID,
		Username:     tok.Username,
		IsGuest:      tok.IsGuest,
	}, nil
}
