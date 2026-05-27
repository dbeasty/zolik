package ssh

import "context"

// Session holds auth tokens for a TUI session.
type Session struct {
	AccessToken  string
	RefreshToken string
	UserID       string
	Username     string
	IsGuest      bool
}

// Authenticator issues sessions for SSH users.
type Authenticator interface {
	Guest(ctx context.Context, guestName string) (Session, error)
	Login(ctx context.Context, username, password string) (Session, error)
}
