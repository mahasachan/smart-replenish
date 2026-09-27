package decision

import (
	"smartreplenish/internal/domain"
	"smartreplenish/internal/replenishment"
	"time"
)

type ContextInput struct {
	User                                  domain.User
	Product                               domain.Product
	Prediction                            replenishment.Prediction
	Membership                            domain.Membership
	CartCount, ListCount                  int
	Pending                               bool
	LastSuggestion, LastProductSuggestion *time.Time
	PurchaseDates                         []time.Time
	Shown, Accepted, Dismissed            int
	LastActivity                          *time.Time
	RunningLowSnoozedUntil                *time.Time
}

func BuildContext(in ContextInput, now time.Time) DecisionContext {
	c := DecisionContext{Version: ContextVersion, AsOf: now,
		User:     UserContext{Timezone: in.User.Timezone, SuggestionEnabled: in.User.SuggestionEnabled, NotificationEnabled: in.User.NotificationEnabled, ShoppingFrequency: "unknown"},
		Product:  ProductContext{Prediction: in.Prediction, Category: in.Product.Category, Available: in.Product.Available, CurrentlyInCart: in.Membership.InCart, CurrentlyInShoppingList: in.Membership.InList, PendingSuggestion: in.Pending},
		Shopping: ShoppingContext{CartItemCount: in.CartCount, SmartListItemCount: in.ListCount}}
	hours := func(t *time.Time) *float64 {
		if t == nil {
			return nil
		}
		v := max(0, now.Sub(*t).Hours())
		return &v
	}
	c.User.LastSuggestionHoursAgo = hours(in.LastSuggestion)
	c.Product.LastSuggestionHoursAgo = hours(in.LastProductSuggestion)
	if in.RunningLowSnoozedUntil != nil && in.RunningLowSnoozedUntil.After(now) {
		v := in.RunningLowSnoozedUntil.Sub(now).Hours()
		c.Product.RunningLowSnoozeHoursRemaining = &v
	}
	if in.Shown > 0 {
		a := float64(in.Accepted) / float64(in.Shown)
		d := float64(in.Dismissed) / float64(in.Shown)
		c.User.SuggestionAcceptanceRate = &a
		c.User.SuggestionDismissRate = &d
	}
	if in.LastActivity != nil {
		age := now.Sub(*in.LastActivity)
		c.Shopping.UserIsCurrentlyShopping = age >= 0 && age <= 30*time.Minute
	}
	loc, err := time.LoadLocation(in.User.Timezone)
	if err != nil {
		loc = time.UTC
	}
	observations := make([]domain.Observation, 0, len(in.PurchaseDates))
	counts := map[time.Weekday]int{}
	seen := map[string]bool{}
	for _, t := range in.PurchaseDates {
		if t.After(now) {
			continue
		}
		observations = append(observations, domain.Observation{PurchasedAt: t, Quantity: 1})
		key := t.In(loc).Format("2006-01-02")
		if !seen[key] {
			counts[t.In(loc).Weekday()]++
			seen[key] = true
		}
	}
	p := replenishment.Predict(domain.Product{ReplenishableScore: 1}, observations, now, loc)
	if p.Eligible {
		switch {
		case p.MedianRepurchaseDays <= 2:
			c.User.ShoppingFrequency = "daily"
		case p.MedianRepurchaseDays <= 10:
			c.User.ShoppingFrequency = "weekly"
		case p.MedianRepurchaseDays <= 20:
			c.User.ShoppingFrequency = "fortnightly"
		default:
			c.User.ShoppingFrequency = "monthly"
		}
	}
	for d := time.Sunday; d <= time.Saturday; d++ {
		if len(seen) >= 3 && counts[d]*2 > len(seen) {
			name := d.String()
			c.User.PreferredShoppingDay = &name
			break
		}
	}
	return c
}
