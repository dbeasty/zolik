package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Claiming the seats a person took at tables with no internet.
//
// A guest at an offline table is a guest of that table's host, not of the
// cloud: the cloud never saw them and has no session to look up. What they
// leave with is a seat receipt, signed by the host node, saying "this guest id
// sat here". When they later sign in, their phone hands those receipts up, and
// the cloud records the guest ids as that account's.
//
// The record has to outlive the sign-in by a long way. The match itself may
// not reach the cloud for weeks - it is sitting in the outbox of a phone in a
// drawer - and when it does arrive it has to be credited to the person who
// played it rather than to a guest nobody can sign in as.

// GuestClaimStore records which account a guest id turned out to belong to.
type GuestClaimStore interface {
	Claim(ctx context.Context, guestID, userID, nodeID, when string) error
	AccountForGuest(ctx context.Context, guestID string) (string, bool, error)
}

type claimOfflineReq struct {
	// Receipts are the seat receipts this device collected at offline tables,
	// each signed by the node that hosted one.
	Receipts []string `json:"receipts"`
}

type claimedSeat struct {
	GuestID string `json:"guestId"`
	NodeID  string `json:"nodeId"`
	// Claimed is false when the receipt was good but the guest already
	// belongs to somebody, which is not an error and not something the person
	// can do anything about.
	Claimed bool   `json:"claimed"`
	Reason  string `json:"reason,omitempty"`
}

// claimOffline takes the seat receipts a signed-in person collected at offline
// tables and records those guest ids as theirs.
//
// Every receipt is checked against the key of the node that issued it, and a
// node nobody enrolled is not trusted to attest anything. A receipt that fails
// is reported rather than failing the whole request: a person with five
// receipts and one bad one should get the other four claimed.
func (h *Handlers) claimOffline(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	uc, _ := GetUserContext(req)
	if uc.IsGuest {
		http.Error(w, "sign in with an account first", http.StatusForbidden)
		return
	}
	if h.guestClaims == nil || h.nodes == nil {
		http.Error(w, "this server does not take offline matches", http.StatusNotImplemented)
		return
	}
	var body claimOfflineReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || len(body.Receipts) == 0 {
		http.Error(w, "receipts required", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	out := make([]claimedSeat, 0, len(body.Receipts))
	for _, receipt := range body.Receipts {
		claims, node, err := VerifySeatReceiptFromNode(ctx, h.nodes, receipt)
		if err != nil {
			out = append(out, claimedSeat{Reason: "this receipt could not be checked"})
			continue
		}
		seat := claimedSeat{GuestID: claims.GuestID, NodeID: node.ID}
		switch err := h.guestClaims.Claim(ctx, claims.GuestID, uc.UserID, node.ID, now); {
		case err == nil:
			seat.Claimed = true
		default:
			// Either somebody else proved it first, or the store refused. The
			// person is told which seat, not why the database said no.
			seat.Reason = "that seat belongs to another account"
		}
		out = append(out, seat)

		// Anything of this guest's that has already arrived is moved now, so
		// the history appears while the person is still looking at the screen
		// rather than whenever the next match happens to be imported.
		if seat.Claimed && h.accounts != nil {
			if u, err := h.store.FindUserByID(ctx, uc.UserID); err == nil {
				if _, err := h.accounts.ClaimGuest(ctx, claims.GuestID, u); err != nil {
					// The claim itself is recorded, so the matches will be
					// credited when they arrive or when this is retried; the
					// person is not told about a bookkeeping hiccup they can
					// do nothing with.
					seat.Reason = ""
				}
			}
		}
	}
	writeJSON(w, map[string]any{"seats": out})
}
