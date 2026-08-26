package match

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// moduleRules needs no database — it only reads a module's own descriptor and
// rules, the same trust level as /modules — so these are plain HTTP tests
// against a bare router, unlike the Mongo-backed tests elsewhere in this
// package.
func rulesRouter() *chi.Mux {
	h := NewHandlers(&Manager{registry: registry()}, false)
	r := chi.NewRouter()
	r.Get("/modules/{id}/rules", h.moduleRules)
	return r
}

func getRules(t *testing.T, url string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	rulesRouter().ServeHTTP(rec, req)
	var body map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("response is not valid JSON: %v (%s)", err, rec.Body.String())
		}
	}
	return rec, body
}

func TestModuleRules_HappyPathReflectsTheOption(t *testing.T) {
	rec, body := getRules(t, "/modules/zolik/rules?variation=zolik_classic&opt.initialMeldMinimum=50")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %v", rec.Code, body)
	}
	sections, ok := body["sections"].([]any)
	if !ok || len(sections) == 0 {
		t.Fatalf("expected non-empty sections, got %v", body)
	}
	found := false
	for _, s := range sections {
		section := s.(map[string]any)
		for _, item := range section["items"].([]any) {
			it := item.(map[string]any)
			if it["labelKey"] == "zolik.rules.meldFloor.on" {
				found = true
				params, _ := it["params"].(map[string]any)
				if params["n"] != float64(50) {
					t.Errorf("meldFloor.on n = %v, want 50", params["n"])
				}
			}
		}
	}
	if !found {
		t.Errorf("expected zolik.rules.meldFloor.on in the response: %v", body)
	}
}

func TestModuleRules_UnknownModule(t *testing.T) {
	rec, body := getRules(t, "/modules/marias/rules")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %v", rec.Code, body)
	}
	if body["code"] != "UNKNOWN_MODULE" {
		t.Errorf("code = %v, want UNKNOWN_MODULE", body["code"])
	}
}

func TestModuleRules_UnknownVariation(t *testing.T) {
	rec, body := getRules(t, "/modules/zolik/rules?variation=does_not_exist")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %v", rec.Code, body)
	}
	if body["code"] != "UNKNOWN_VARIATION" {
		t.Errorf("code = %v, want UNKNOWN_VARIATION", body["code"])
	}
}

func TestModuleRules_UndeclaredOptionValueIsRejected(t *testing.T) {
	// 999 is not one of the descriptor's declared choices for this option.
	rec, body := getRules(t, "/modules/zolik/rules?opt.initialMeldMinimum=999")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %v", rec.Code, body)
	}
}

func TestModuleRules_NonIntegerOptionIsRejected(t *testing.T) {
	rec, body := getRules(t, "/modules/zolik/rules?opt.initialMeldMinimum=fifty")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %v", rec.Code, body)
	}
	if body["code"] != "BAD_OPTION" {
		t.Errorf("code = %v, want BAD_OPTION", body["code"])
	}
}
