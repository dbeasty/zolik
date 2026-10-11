package ginrummy

import (
	"strings"
	"testing"
)

// A finished hand's caption is worded by the client from a key: the bare
// engine word ("knock") used to reach the screen as it was.
func TestEveryWayAHandEndsHasAWordingKey(t *testing.T) {
	for _, kind := range []string{"gin", "knock", "undercut", "dead"} {
		if got := roundKindKey(kind); !strings.HasPrefix(got, "ginrummy.kind.") {
			t.Errorf("%s ends a hand but reads %q", kind, got)
		}
	}
}
