package app

import (
	"context"
	"encoding/json"
	"smartreplenish/internal/domain"
)

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
			if status == "accepted" {
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
