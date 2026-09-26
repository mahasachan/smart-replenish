package policy

import (
	"math"
	"smartreplenish/internal/decision"
)

type Result struct {
	Allowed          bool                      `json:"allowed"`
	Action           decision.Action           `json:"action"`
	Reason           string                    `json:"reason"`
	ExecutionContext *decision.DecisionContext `json:"execution_context,omitempty"`
}
type PolicyEngine struct{}

func (PolicyEngine) Validate(r decision.DecisionResult, c decision.DecisionContext) Result {
	block := func(a decision.Action, reason string) Result { return Result{Action: a, Reason: reason} }
	if !r.Action.Valid() {
		return block(decision.DoNothing, "invalid_action")
	}
	if math.IsNaN(r.Confidence) || math.IsInf(r.Confidence, 0) || r.Confidence < 0 || r.Confidence > 1 {
		return block(decision.DoNothing, "invalid_confidence")
	}
	if r.ContextVersion != decision.ContextVersion || c.Version != decision.ContextVersion {
		return block(decision.DoNothing, "unsupported_context_version")
	}
	if !c.User.SuggestionEnabled {
		return block(decision.DoNothing, "suggestions_disabled")
	}
	if !c.Product.Available {
		return block(decision.DoNothing, "unavailable")
	}
	if c.Product.CurrentlyInCart {
		return block(decision.DoNothing, "already_in_cart")
	}
	if c.Product.CurrentlyInShoppingList {
		return block(decision.DoNothing, "already_in_list")
	}
	if !c.Product.Eligible {
		return block(decision.DoNothing, "ineligible")
	}
	if r.Action.CreatesSuggestion() {
		if c.Product.LastSuggestionHoursAgo != nil && *c.Product.LastSuggestionHoursAgo < 12 {
			return block(decision.Wait, "product_cooldown")
		}
		if c.Product.PendingSuggestion {
			return block(decision.Wait, "pending_suggestion")
		}
	}
	return Result{Allowed: true, Action: r.Action, Reason: "allowed"}
}
