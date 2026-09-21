package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"zolik/server/internal/module"
)

// A sign-in the database shed under memory pressure was answered with a bare
// 500, so a new visitor to a busy server saw "internal server error" instead
// of the "server is full, try again" every other refused request gets.
func TestInternalErrorAnswersAShedWriteAsBusy(t *testing.T) {
	shed := fmt.Errorf("creating guest: %w", module.Error{Code: "SERVER_BUSY", Message: "memory pressure zone high"})
	rec := httptest.NewRecorder()
	internalError(rec, "guest", shed)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", rec.Code)
	}
	var body struct{ Code string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != "SERVER_BUSY" {
		t.Fatalf("body %q, want code SERVER_BUSY", rec.Body.String())
	}
}

func TestInternalErrorStillHidesOtherFailures(t *testing.T) {
	rec := httptest.NewRecorder()
	internalError(rec, "guest", errors.New("disk on fire"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", rec.Code)
	}
}
