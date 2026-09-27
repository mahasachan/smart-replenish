package policy

import (
	"smartreplenish/internal/decision"
	"smartreplenish/internal/replenishment"
	"testing"
)

func TestPolicyBlocks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*decision.DecisionContext, *decision.DecisionResult)
		reason string
	}{
		{"disabled", func(c *decision.DecisionContext, r *decision.DecisionResult) { c.User.SuggestionEnabled = false }, "suggestions_disabled"},
		{"cart", func(c *decision.DecisionContext, r *decision.DecisionResult) { c.Product.CurrentlyInCart = true }, "already_in_cart"},
		{"list", func(c *decision.DecisionContext, r *decision.DecisionResult) {
			c.Product.CurrentlyInShoppingList = true
		}, "already_in_list"},
		{"unavailable", func(c *decision.DecisionContext, r *decision.DecisionResult) { c.Product.Available = false }, "unavailable"},
		{"cooldown", func(c *decision.DecisionContext, r *decision.DecisionResult) {
			v := 11.999
			c.Product.LastSuggestionHoursAgo = &v
		}, "product_cooldown"},
		{"pending", func(c *decision.DecisionContext, r *decision.DecisionResult) { c.Product.PendingSuggestion = true }, "pending_suggestion"},
		{"action", func(c *decision.DecisionContext, r *decision.DecisionResult) { r.Action = "BUY_NOW" }, "invalid_action"},
		{"confidence", func(c *decision.DecisionContext, r *decision.DecisionResult) { r.Confidence = 2 }, "invalid_confidence"},
		{"version", func(c *decision.DecisionContext, r *decision.DecisionResult) { r.ContextVersion = "v1" }, "unsupported_context_version"},
		{"snoozed", func(c *decision.DecisionContext, r *decision.DecisionResult) {
			v := 0.5
			c.Product.RunningLowSnoozeHoursRemaining = &v
			r.Action = decision.AskIfRunningLow
		}, "running_low_snoozed"},
		{"ineligible", func(c *decision.DecisionContext, r *decision.DecisionResult) { c.Product.Eligible = false }, "ineligible"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := decision.DecisionContext{Version: decision.ContextVersion, User: decision.UserContext{SuggestionEnabled: true}, Product: decision.ProductContext{Prediction: replenishment.Prediction{Eligible: true}, Available: true}}
			r := decision.DecisionResult{Action: decision.SuggestNow, Confidence: .8, ContextVersion: decision.ContextVersion}
			tc.change(&c, &r)
			p := (PolicyEngine{}).Validate(r, c)
			if p.Allowed || p.Reason != tc.reason || p.Action.CreatesSuggestion() {
				t.Fatalf("%+v", p)
			}
		})
	}
	c := decision.DecisionContext{Version: decision.ContextVersion, User: decision.UserContext{SuggestionEnabled: true}, Product: decision.ProductContext{Prediction: replenishment.Prediction{Eligible: true}, Available: true}}
	v := 12.0
	c.Product.LastSuggestionHoursAgo = &v
	if p := (PolicyEngine{}).Validate(decision.DecisionResult{Action: decision.SuggestNow, Confidence: .8, ContextVersion: decision.ContextVersion}, c); !p.Allowed {
		t.Fatalf("12h must be allowed: %+v", p)
	}
}
