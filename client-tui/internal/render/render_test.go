package render

import (
	"strings"
	"testing"
)

func TestRenderCard_TenOfHearts(t *testing.T) {
	out := RenderCard("TH", false, false)
	if !strings.Contains(out, "10") {
		t.Fatalf("expected rank 10 in output: %q", out)
	}
	if !strings.Contains(out, "♥") {
		t.Fatalf("expected heart suit: %q", out)
	}
}

func TestRenderCard_Joker(t *testing.T) {
	out := RenderCard("JOKER1", false, false)
	if !strings.Contains(out, "JKR") {
		t.Fatalf("expected JKR: %q", out)
	}
}

func TestRenderCard_SelectedDoubleBorder(t *testing.T) {
	out := RenderCard("7H", true, false)
	if !strings.Contains(out, "║") && !strings.Contains(out, "╔") {
		t.Fatalf("expected double border chars in selected card: %q", out)
	}
}

func TestRenderHandCompact(t *testing.T) {
	out := RenderHandCompact([]string{"AH", "KS"}, []int{1})
	if !strings.Contains(out, "[A") || !strings.Contains(out, "[K") {
		t.Fatalf("unexpected compact hand: %q", out)
	}
}

func TestUseCompact(t *testing.T) {
	if !UseCompact(79) {
		t.Fatal("expected compact at 79 cols")
	}
	if UseCompact(80) {
		t.Fatal("expected full at 80 cols")
	}
}
