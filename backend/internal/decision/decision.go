package decision

import (
	"context"
	"encoding/json"
	"smartreplenish/internal/replenishment"
	"time"
)

type Action string

const (
	DoNothing                Action = "DO_NOTHING"
	Wait                     Action = "WAIT"
	SuggestNow               Action = "SUGGEST_NOW"
	SuggestBundle            Action = "SUGGEST_BUNDLE"
	AddToSmartListSuggestion Action = "ADD_TO_SMART_LIST_SUGGESTION"
	AskIfRunningLow          Action = "ASK_IF_RUNNING_LOW"
	ContextVersion                  = "v1"
)

func (a Action) Valid() bool {
	switch a {
	case DoNothing, Wait, SuggestNow, SuggestBundle, AddToSmartListSuggestion, AskIfRunningLow:
		return true
	}
	return false
}
func (a Action) CreatesSuggestion() bool { return a.Valid() && a != DoNothing && a != Wait }

type UserContext struct {
	Timezone                 string   `json:"timezone"`
	SuggestionEnabled        bool     `json:"suggestion_enabled"`
	NotificationEnabled      bool     `json:"notification_enabled"`
	ShoppingFrequency        string   `json:"shopping_frequency"`
	PreferredShoppingDay     *string  `json:"preferred_shopping_day"`
	SuggestionAcceptanceRate *float64 `json:"suggestion_acceptance_rate"`
	SuggestionDismissRate    *float64 `json:"suggestion_dismiss_rate"`
	LastSuggestionHoursAgo   *float64 `json:"last_suggestion_hours_ago"`
}
type ProductContext struct {
	replenishment.Prediction
	Category                string   `json:"category"`
	Available               bool     `json:"available"`
	CurrentlyInCart         bool     `json:"currently_in_cart"`
	CurrentlyInShoppingList bool     `json:"currently_in_shopping_list"`
	PendingSuggestion       bool     `json:"pending_suggestion"`
	LastSuggestionHoursAgo  *float64 `json:"last_suggestion_hours_ago"`
}
type ShoppingContext struct {
	CartItemCount           int  `json:"cart_item_count"`
	SmartListItemCount      int  `json:"smart_list_item_count"`
	UserIsCurrentlyShopping bool `json:"user_is_currently_shopping"`
}
type DecisionContext struct {
	Version  string          `json:"decision_context_version"`
	AsOf     time.Time       `json:"as_of"`
	User     UserContext     `json:"user"`
	Product  ProductContext  `json:"product"`
	Shopping ShoppingContext `json:"shopping_context"`
}
type DecisionResult struct {
	Action         Action          `json:"action"`
	Confidence     float64         `json:"confidence"`
	Probability    *float64        `json:"probability,omitempty"`
	Provider       string          `json:"provider"`
	Model          string          `json:"model"`
	ContextVersion string          `json:"decision_context_version"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	LatencyMS      int64           `json:"latency_ms"`
	CostUSD        *float64        `json:"cost_usd"`
}
type DecisionEngine interface {
	Decide(context.Context, DecisionContext) (DecisionResult, error)
}
type RuleBasedDecisionEngine struct{}

func (RuleBasedDecisionEngine) Decide(_ context.Context, c DecisionContext) (DecisionResult, error) {
	a := Wait
	if c.Product.Eligible && c.Product.EstimatedDaysRemaining != nil && *c.Product.EstimatedDaysRemaining <= 2 && c.Product.PredictionConfidence >= 0.7 {
		a = SuggestNow
	}
	return DecisionResult{Action: a, Confidence: c.Product.PredictionConfidence, Provider: "rules", Model: "median-v1", ContextVersion: c.Version}, nil
}
