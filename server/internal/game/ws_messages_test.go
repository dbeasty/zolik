package game

import (
	"testing"

	"zolik/server/internal/rules"
)

func TestDecodeIncoming_JSON(t *testing.T) {
	in, err := DecodeIncoming([]byte(`{"type":"draw_card","from":"deck"}`))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if in.Type != "draw_card" || in.From != "deck" {
		t.Fatalf("unexpected decode: %#v", in)
	}
}

func TestToRulesAction_MappingDiscard(t *testing.T) {
	action, err := toRulesAction(WSIncoming{Type: "discard", Card: "KH"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if action.Type != rules.ActionDiscard {
		t.Fatalf("expected discard action type")
	}
	if action.Card != "KH" {
		t.Fatalf("expected card KH")
	}
}

