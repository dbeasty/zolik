package ui

type PlayerSession struct {
	AccessToken  string
	RefreshToken string
	UserID       string
	Username     string
	IsGuest      bool
}
