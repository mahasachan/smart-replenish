package app

import (
	"smartreplenish/internal/decision"
	"smartreplenish/internal/replenishment"
	"sort"
	"time"
)

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
		switch e.Type {
		case "PRODUCT_VIEWED", "ADDED_TO_CART", "REMOVED_FROM_CART", "ADDED_TO_LIST":
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
		out = append(out, decision.BuildContext(in, now))
	}
	return out
}
