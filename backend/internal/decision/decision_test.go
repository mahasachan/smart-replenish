package decision

import (
	"context"
	"smartreplenish/internal/domain"
	"smartreplenish/internal/replenishment"
	"testing"
	"time"
)

func TestRuleBoundaries(t *testing.T) {
	for _, tc := range []struct {
		days, confidence float64
		eligible         bool
		want             Action
	}{{2, .7, true, SuggestNow}, {2.01, .7, true, Wait}, {2, .699, true, Wait}, {-4, .9, true, SuggestNow}, {-4, .9, false, Wait}} {
		c := DecisionContext{Version: ContextVersion, Product: ProductContext{Prediction: replenishment.Prediction{Eligible: tc.eligible, EstimatedDaysRemaining: &tc.days, PredictionConfidence: tc.confidence}}}
		r, err := (RuleBasedDecisionEngine{}).Decide(context.Background(), c)
		if err != nil || r.Action != tc.want {
			t.Fatalf("%+v: %+v %v", tc, r, err)
		}
	}
}
func TestContextUnknownAndObservedFacts(t *testing.T) {
	now := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	in := ContextInput{User: domain.User{Timezone: "Asia/Bangkok", SuggestionEnabled: true}, Product: domain.Product{Category: "milk", Available: true}, Prediction: replenishment.Prediction{ProductID: "milk"}, Membership: domain.Membership{InCart: true}, CartCount: 4, ListCount: 3}
	c := BuildContext(in, now)
	if c.Version != "v1" || c.User.SuggestionAcceptanceRate != nil || c.User.LastSuggestionHoursAgo != nil || c.User.ShoppingFrequency != "unknown" || c.User.PreferredShoppingDay != nil {
		t.Fatalf("invented facts: %+v", c)
	}
	last := now.Add(-32 * time.Hour)
	activity := now.Add(-10 * time.Minute)
	in.LastSuggestion = &last
	in.LastProductSuggestion = &last
	in.LastActivity = &activity
	in.Shown = 10
	in.Accepted = 6
	in.Dismissed = 2
	in.PurchaseDates = []time.Time{now.AddDate(0, 0, -21), now.AddDate(0, 0, -14), now.AddDate(0, 0, -7)}
	c = BuildContext(in, now)
	if *c.User.SuggestionAcceptanceRate != .6 || *c.User.SuggestionDismissRate != .2 || *c.Product.LastSuggestionHoursAgo != 32 || !c.Product.CurrentlyInCart || !c.Shopping.UserIsCurrentlyShopping || c.User.ShoppingFrequency != "weekly" || *c.User.PreferredShoppingDay != "Monday" {
		t.Fatalf("missing facts: %+v", c)
	}
}
