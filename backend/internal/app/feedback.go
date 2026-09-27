package app

import (
	"context"
	"encoding/json"
	"math"
	"smartreplenish/internal/decision"
	"smartreplenish/internal/domain"
	"smartreplenish/internal/replenishment"
	"time"
)

// AnswerRunningLow records the user's answer to a running-low question.
// Yes accepts it and adds the product to the cart; no dismisses it and starts the snooze.
func (s *Service) AnswerRunningLow(ctx context.Context, id string, runningLow bool) (domain.Suggestion, error) {
	sg, err := s.Store.Suggestion(ctx, id)
	if err != nil {
		return sg, err
	}
	if sg.Kind != string(decision.AskIfRunningLow) {
		return sg, invalid("suggestion is not a running-low question")
	}
	if runningLow {
		return s.Feedback(ctx, id, "accept")
	}
	return s.Feedback(ctx, id, "dismiss")
}

// CartQuantity is the median whole-unit quantity per purchase occasion, at least one.
func CartQuantity(history []domain.Observation, now time.Time, loc *time.Location) int64 {
	q := int64(math.Round(replenishment.Predict(domain.Product{}, history, now, loc).MedianQuantity))
	return min(domain.MaxCartQuantity, max(1, q))
}

// addConfirmedToCart never lowers a quantity the user already chose and never checks out.
func addConfirmedToCart(ctx context.Context, tx Transaction, f Facts, user, suggestion string, item domain.SuggestionItem, now time.Time) error {
	m := f.Membership[item.ProductID]
	if m.InCart {
		return nil
	}
	loc, err := time.LoadLocation(f.User.Timezone)
	if err != nil {
		loc = time.UTC
	}
	m.InCart = true
	m.AutoAdded = true
	m.CartQuantity = CartQuantity(f.History[item.ProductID], now, loc)
	if err = tx.SaveMembership(ctx, item.ProductID, m); err != nil {
		return err
	}
	product, decisionID := item.ProductID, item.DecisionID
	metadata, _ := json.Marshal(map[string]any{"source": "running_low_confirmed", "quantity": m.CartQuantity})
	return tx.SaveEvent(ctx, domain.Event{ID: domain.NewID(), UserID: user, ProductID: &product, DecisionID: &decisionID, SuggestionID: &suggestion, Type: "ADDED_TO_CART", Metadata: metadata, CreatedAt: now})
}

func (s *Service) Feedback(ctx context.Context, id, verb string) (domain.Suggestion, error) {
	initial, err := s.Store.Suggestion(ctx, id)
	if err != nil {
		return initial, err
	}
	var result domain.Suggestion
	err = s.Store.WithUser(ctx, initial.UserID, func(tx Transaction) error {
		f, err := tx.Facts(ctx)
		if err != nil {
			return err
		}
		found := false
		for _, sg := range f.Suggestions {
			if sg.ID == id {
				result = sg
				found = true
				break
			}
		}
		if !found {
			return domain.ErrNotFound
		}
		status := ""
		kind := ""
		switch verb {
		case "shown":
			kind = "SUGGESTION_SHOWN"
		case "accept":
			status = "accepted"
			kind = "SUGGESTION_ACCEPTED"
		case "dismiss":
			status = "dismissed"
			kind = "SUGGESTION_DISMISSED"
		default:
			return invalid("unsupported feedback")
		}
		if status != "" && result.Status != "pending" {
			if result.Status == status {
				return nil
			}
			return domain.ErrConflict
		}
		now := s.now()
		for _, item := range result.Items {
			product, decisionID := item.ProductID, item.DecisionID
			event := domain.Event{ID: domain.NewID(), UserID: result.UserID, ProductID: &product, DecisionID: &decisionID, SuggestionID: &id, Type: "SUGGESTION_SHOWN", Metadata: json.RawMessage(`{}`), CreatedAt: now}
			if err = tx.SaveEvent(ctx, event); err != nil {
				return err
			}
			if kind != "SUGGESTION_SHOWN" {
				event.ID = domain.NewID()
				event.Type = kind
				if err = tx.SaveEvent(ctx, event); err != nil {
					return err
				}
			}
			if status == "accepted" && result.Kind == string(decision.AskIfRunningLow) {
				if err = addConfirmedToCart(ctx, tx, f, result.UserID, id, item, now); err != nil {
					return err
				}
			} else if status == "accepted" {
				m := f.Membership[product]
				if !m.InList {
					m.InList = true
					if err = tx.SaveMembership(ctx, product, m); err != nil {
						return err
					}
					if err = tx.SaveEvent(ctx, domain.Event{ID: domain.NewID(), UserID: result.UserID, ProductID: &product, Type: "ADDED_TO_LIST", Metadata: json.RawMessage(`{}`), CreatedAt: now}); err != nil {
						return err
					}
				}
			}
		}
		if status != "" {
			result.Status = status
			return tx.SetSuggestionStatus(ctx, id, status)
		}
		return nil
	})
	return result, err
}
