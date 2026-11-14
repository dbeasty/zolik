package auth

import (
	"context"
	"testing"
	"time"

	"zolik/server/internal/db"
)

// A claim is good only for matches from the node whose receipt proved it: a
// guest id lives in one node's namespace, so another node naming it in a
// handed-up match is inventing a seat in a stranger's history.
func TestGuestClaimCreditsOnlyTheProvingNode(t *testing.T) {
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	c := NewKDBGuestClaims(k)
	ctx := context.Background()
	const guest = "b8f0c0ffeec0ffeeb8f0c0ffeec0ffee"
	if err := c.Claim(ctx, guest, "u1", "node-a", time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if account, ok, err := c.AccountForGuestAt(ctx, guest, "node-a"); err != nil || !ok || account != "u1" {
		t.Fatalf("from the proving node: %q %v %v, want u1", account, ok, err)
	}
	if account, ok, _ := c.AccountForGuestAt(ctx, guest, "node-b"); ok {
		t.Fatalf("a match from another node was credited to %q", account)
	}
}
