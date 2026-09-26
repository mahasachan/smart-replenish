package action

import (
	"fmt"
	"smartreplenish/internal/decision"
	"smartreplenish/internal/domain"
	"strings"
	"time"
)

type Item struct {
	Product    domain.Product
	DecisionID string
}

// Build constructs an inert proposal. The application persists it in its transaction.
func Build(userID string, a decision.Action, items []Item, now time.Time) (*domain.Suggestion, decision.Action) {
	if !a.CreatesSuggestion() || len(items) == 0 {
		return nil, decision.DoNothing
	}
	if a == decision.SuggestBundle && len(items) == 1 {
		a = decision.SuggestNow
	}
	s := &domain.Suggestion{ID: domain.NewID(), UserID: userID, Kind: string(a), Status: "pending", Section: "suggestions", CreatedAt: now, Items: []domain.SuggestionItem{}}
	names := make([]string, 0, len(items))
	for _, i := range items {
		names = append(names, i.Product.Name)
		s.Items = append(s.Items, domain.SuggestionItem{ProductID: i.Product.ID, DecisionID: i.DecisionID})
	}
	switch a {
	case decision.SuggestNow:
		s.Message = fmt.Sprintf("%s may be running low. Add it to your list?", names[0])
	case decision.SuggestBundle:
		s.Message = "Items you may need soon: " + strings.Join(names, ", ") + ". Add them to your list?"
	case decision.AddToSmartListSuggestion:
		s.Message = names[0] + " may be needed soon."
		s.Section = "suggested_list"
	case decision.AskIfRunningLow:
		s.Message = "Are you running low on " + names[0] + "?"
	}
	return s, a
}
