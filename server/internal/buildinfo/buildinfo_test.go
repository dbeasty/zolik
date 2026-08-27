package buildinfo

import "testing"

// TestResolved_FallsBackWhenUnlinked guards against a -X flag that silently
// no-ops (a typo'd import path, or a var the linker dead-code-eliminates —
// both link cleanly with the var left empty, with no error). An unstamped
// build must still report something, never a blank string, since the
// clients render this directly.
func TestResolved_FallsBackWhenUnlinked(t *testing.T) {
	origVersion, origCommit := Version, Commit
	t.Cleanup(func() { Version, Commit = origVersion, origCommit })
	Version, Commit = "", ""

	version, commit := Resolved()

	if version == "" {
		t.Error("Resolved() version is empty; want a fallback like 0.0.0-dev")
	}
	if commit == "" {
		t.Error("Resolved() commit is empty; want a fallback like unknown")
	}
}

// TestResolved_PrefersLinkedValues confirms the -X-set values are used
// verbatim when present, without falling back to VCS info.
func TestResolved_PrefersLinkedValues(t *testing.T) {
	origVersion, origCommit := Version, Commit
	t.Cleanup(func() { Version, Commit = origVersion, origCommit })
	Version, Commit = "1.1.1.2", "7feb025"

	version, commit := Resolved()

	if version != "1.1.1.2" {
		t.Errorf("Resolved() version = %q, want 1.1.1.2", version)
	}
	if commit != "7feb025" {
		t.Errorf("Resolved() commit = %q, want 7feb025", commit)
	}
}
