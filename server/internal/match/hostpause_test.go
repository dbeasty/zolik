package match_test

import (
	"testing"
	"time"
)

// While a phone host cannot reach its remote guests, nobody's seat is given
// to a bot for being away — they cannot be anywhere else. When it can again,
// every absence starts afresh and the ordinary wait applies.
func TestStandInsWaitWhileTheHostIsCutOff(t *testing.T) {
	ctx := t.Context()
	m, id, gone, stays := standInTable(t, nil)
	sitDown(t, m, id, stays)
	leave := sitDown(t, m, id, gone)

	m.HoldStandIns(true)
	leave()
	m.SuspendOnDisconnect(ctx, id, gone, "test")
	time.Sleep(500 * time.Millisecond)
	if seat(t, m, id, gone).StandIn != nil {
		t.Fatal("a bot took a seat while the host could not reach its player")
	}

	released := time.Now()
	m.HoldStandIns(false)
	eventually(t, "the stand-in once the host is reachable again", func() bool {
		return seat(t, m, id, gone).StandIn != nil
	})
	// A fresh start: not a moment before the ordinary wait since release.
	if since := seat(t, m, id, gone).StandIn.Since; since.Sub(released) < 100*time.Millisecond {
		t.Errorf("the stand-in came %s after release; the wait should have started again", since.Sub(released))
	}
}

// A host back from the background charges nobody for its own absence.
func TestAHostComingBackRestartsTheClocks(t *testing.T) {
	ctx := t.Context()
	m, id, gone, stays := standInTable(t, nil)
	m.SetStandInWait(400 * time.Millisecond)
	sitDown(t, m, id, stays)
	leave := sitDown(t, m, id, gone)
	leave()
	m.SuspendOnDisconnect(ctx, id, gone, "test")
	time.Sleep(250 * time.Millisecond)

	resumed := time.Now()
	m.HostResumed()
	eventually(t, "the stand-in after the fresh wait", func() bool {
		return seat(t, m, id, gone).StandIn != nil
	})
	if since := seat(t, m, id, gone).StandIn.Since; since.Sub(resumed) < 300*time.Millisecond {
		t.Errorf("the stand-in came %s after the host came back; the full wait should run again", since.Sub(resumed))
	}
}
