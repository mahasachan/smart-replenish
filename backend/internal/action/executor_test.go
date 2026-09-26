package action

import (
	"smartreplenish/internal/decision"
	"smartreplenish/internal/domain"
	"testing"
	"time"
)

func TestBoundedActions(t *testing.T) {
	items := []Item{{Product: domain.Product{ID: "milk", Name: "Milk"}, DecisionID: "d1"}, {Product: domain.Product{ID: "eggs", Name: "Eggs"}, DecisionID: "d2"}}
	for _, a := range []decision.Action{decision.DoNothing, decision.Wait, "BUY_NOW"} {
		if s, _ := Build("u", a, items, time.Now()); s != nil {
			t.Fatal("unexpected side effect")
		}
	}
	s, a := Build("u", decision.SuggestBundle, items, time.Now())
	if a != decision.SuggestBundle || len(s.Items) != 2 || s.Items[1].DecisionID != "d2" {
		t.Fatalf("%+v", s)
	}
	_, a = Build("u", decision.SuggestBundle, items[:1], time.Now())
	if a != decision.SuggestNow {
		t.Fatal("singleton bundle not degraded")
	}
	s, _ = Build("u", decision.AddToSmartListSuggestion, items[:1], time.Now())
	if s.Section != "suggested_list" || s.Status != "pending" {
		t.Fatalf("%+v", s)
	}
}
