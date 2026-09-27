package app

import (
	"smartreplenish/internal/decision"
	"smartreplenish/internal/replenishment"
	"sort"
	"time"
)

// RunningLowSnooze is how long a "not yet" answer suppresses new running-low questions:
// half the product's median repurchase interval, never less than one day.
func RunningLowSnooze(medianRepurchaseDays float64) time.Duration {
	return max(24*time.Hour, time.Duration(medianRepurchaseDays/2*24*float64(time.Hour)))
}

// deniedRunningLow returns, per product, the latest time the user answered "not yet".
func deniedRunningLow(f Facts, now time.Time) map[string]time.Time {
	asks := map[string]bool{}
	for _, sg := range f.Suggestions {
		if sg.Kind == string(decision.AskIfRunningLow) {
			asks[sg.ID] = true
		}
	}
	out := map[string]time.Time{}
	for _, e := range f.Events {
		if e.Type != "SUGGESTION_DISMISSED" || e.SuggestionID == nil || e.ProductID == nil || !asks[*e.SuggestionID] || e.CreatedAt.After(now) {
			continue
		}
		if e.CreatedAt.After(out[*e.ProductID]) {
			out[*e.ProductID] = e.CreatedAt
		}
	}
	return out
}
func contexts(f Facts, now time.Time) []decision.DecisionContext {
	loc, err := time.LoadLocation(f.User.Timezone)
	if err != nil {
		loc = time.UTC
	}
	base := decision.ContextInput{User: f.User, PurchaseDates: f.PurchaseDates}
	for _, m := range f.Membership {
		if m.InCart {
			base.CartCount++
		}
		if m.InList {
			base.ListCount++
		}
	}
	shown, accepted, dismissed := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, e := range f.Events {
		if e.CreatedAt.After(now) {
			continue
		}
		if e.DecisionID != nil {
			switch e.Type {
			case "SUGGESTION_SHOWN":
				shown[*e.DecisionID] = true
			case "SUGGESTION_ACCEPTED":
				accepted[*e.DecisionID] = true
			case "SUGGESTION_DISMISSED":
				dismissed[*e.DecisionID] = true
			}
		}
		// Decision-linked cart additions come from answering a question, not from browsing.
		switch e.Type {
		case "PRODUCT_VIEWED", "ADDED_TO_CART", "REMOVED_FROM_CART", "ADDED_TO_LIST":
			if e.DecisionID != nil {
				continue
			}
			if base.LastActivity == nil || e.CreatedAt.After(*base.LastActivity) {
				t := e.CreatedAt
				base.LastActivity = &t
			}
		}
	}
	base.Shown = len(shown)
	for id := range shown {
		if accepted[id] {
			base.Accepted++
		}
		if dismissed[id] {
			base.Dismissed++
		}
	}
	for _, sg := range f.Suggestions {
		if !sg.CreatedAt.After(now) && (base.LastSuggestion == nil || sg.CreatedAt.After(*base.LastSuggestion)) {
			t := sg.CreatedAt
			base.LastSuggestion = &t
		}
	}
	ids := make([]string, 0, len(f.History))
	for id := range f.History {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	denied := deniedRunningLow(f, now)
	out := make([]decision.DecisionContext, 0, len(ids))
	for _, id := range ids {
		in := base
		in.Product = f.Products[id]
		in.Prediction = replenishment.Predict(in.Product, f.History[id], now, loc)
		in.Membership = f.Membership[id]
		for _, sg := range f.Suggestions {
			if sg.CreatedAt.After(now) {
				continue
			}
			for _, item := range sg.Items {
				if item.ProductID == id {
					if in.LastProductSuggestion == nil || sg.CreatedAt.After(*in.LastProductSuggestion) {
						t := sg.CreatedAt
						in.LastProductSuggestion = &t
					}
					if sg.Status == "pending" {
						in.Pending = true
					}
				}
			}
		}
		if at, ok := denied[id]; ok && in.Prediction.Eligible {
			until := at.Add(RunningLowSnooze(in.Prediction.MedianRepurchaseDays))
			in.RunningLowSnoozedUntil = &until
		}
		out = append(out, decision.BuildContext(in, now))
	}
	return out
}
