package auth

import (
	"context"
	"time"

	"golang.org/x/crypto/bcrypt"

	"zolik/server/internal/models"
)

// SessionTokens is returned after guest login or password login for SSH/TUI clients.
type SessionTokens struct {
	AccessToken  string
	RefreshToken string
	UserID       string
	Username     string
	IsGuest      bool
}

func (h *Handlers) GuestSession(ctx context.Context, guestName string) (SessionTokens, error) {
	if guestName == "" {
		guestName = "Guest"
	}
	refreshToken, err := CreateRefreshToken()
	if err != nil {
		return SessionTokens{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(30 * 24 * time.Hour)

	if err := h.sessionRepo.CreateSession(ctx, models.Session{
		Token:     refreshToken,
		GuestName: guestName,
		UserID:    "",
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}); err != nil {
		return SessionTokens{}, err
	}

	accessToken, err := CreateAccessToken(refreshToken, guestName, true, 7*24*time.Hour)
	if err != nil {
		return SessionTokens{}, err
	}
	return SessionTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       refreshToken,
		Username:     guestName,
		IsGuest:      true,
	}, nil
}

func (h *Handlers) LoginSession(ctx context.Context, username, password string) (SessionTokens, error) {
	u, err := h.findUserByUsername(ctx, username)
	if err != nil {
		return SessionTokens{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return SessionTokens{}, err
	}

	refreshToken, err := CreateRefreshToken()
	if err != nil {
		return SessionTokens{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(30 * 24 * time.Hour)

	if err := h.sessionRepo.CreateSession(ctx, models.Session{
		Token:     refreshToken,
		GuestName: u.Username,
		UserID:    u.ID.Hex(),
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}); err != nil {
		return SessionTokens{}, err
	}

	accessToken, err := CreateAccessToken(u.ID.Hex(), u.Username, false, 15*time.Minute)
	if err != nil {
		return SessionTokens{}, err
	}
	return SessionTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       u.ID.Hex(),
		Username:     u.Username,
		IsGuest:      false,
	}, nil
}
