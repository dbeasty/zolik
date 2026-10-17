package models_test

import (
	"testing"

	"zolik/server/internal/db/doccodec/doccodectest"
	"zolik/server/internal/models"
)

// Every document this package stores on the kdb backend reads the same through
// doccodec as it did through the driver's Extended JSON codec, from a legacy row
// and from a plain one. A type added to what this package stores belongs here.
func TestStoredDocumentsReadTheSameThroughDoccodec(t *testing.T) {
	doccodectest.Check(t,
		models.Identity{}, models.LoginCode{}, models.OAuthFlow{}, models.OAuthFlowResult{},
		models.Match{}, models.RematchRef{}, models.Reservation{}, models.MatchAction{},
		models.Player{}, models.StandIn{}, models.User{}, models.UserPreferences{}, models.Session{},
	)
}
