package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"smartreplenish/internal/domain"
	"sort"
	"strings"
	"time"
)

func invalid(message string) error { return fmt.Errorf("%w: %s", domain.ErrInvalid, message) }
func (s *Service) CreateUser(ctx context.Context, u domain.User) (domain.User, error) {
	if _, err := time.LoadLocation(u.Timezone); err != nil || u.Timezone == "" || u.Timezone == "Local" {
		return u, invalid("timezone must be an IANA timezone")
	}
	u.ID = domain.NewID()
	u.CreatedAt = s.now()
	return u, s.Store.CreateUser(ctx, u)
}
func validProduct(p domain.Product) error {
	if strings.TrimSpace(p.SKU) == "" || len(p.SKU) > 100 || strings.TrimSpace(p.Name) == "" || len(p.Name) > 200 || p.Category == "" || p.Unit == "" || len(p.Category) > 100 || len(p.Brand) > 100 || len(p.Unit) > 100 {
		return invalid("product requires bounded sku, name, category, and unit")
	}
	if math.IsNaN(p.ReplenishableScore) || math.IsInf(p.ReplenishableScore, 0) || p.ReplenishableScore < 0 || p.ReplenishableScore > 1 {
		return invalid("replenishable_score must be between 0 and 1")
	}
	return nil
}
func (s *Service) CreateProduct(ctx context.Context, p domain.Product) (domain.Product, error) {
	if err := validProduct(p); err != nil {
		return p, err
	}
	p.ID = domain.NewID()
	return p, s.Store.CreateProduct(ctx, p)
}

type UserPatch struct {
	Timezone            *string `json:"timezone"`
	NotificationEnabled *bool   `json:"notification_enabled"`
	SuggestionEnabled   *bool   `json:"suggestion_enabled"`
}

func (s *Service) UpdateUser(ctx context.Context, id string, p UserPatch) (domain.User, error) {
	var u domain.User
	err := s.Store.WithUser(ctx, id, func(tx Transaction) error {
		f, err := tx.Facts(ctx)
		if err != nil {
			return err
		}
		u = f.User
		if p.Timezone != nil {
			if _, err = time.LoadLocation(*p.Timezone); err != nil || *p.Timezone == "" || *p.Timezone == "Local" {
				return invalid("timezone must be an IANA timezone")
			}
			u.Timezone = *p.Timezone
		}
		if p.NotificationEnabled != nil {
			u.NotificationEnabled = *p.NotificationEnabled
		}
		if p.SuggestionEnabled != nil {
			u.SuggestionEnabled = *p.SuggestionEnabled
		}
		return tx.SaveUser(ctx, u)
	})
	return u, err
}

type ProductPatch struct {
	Available          *bool    `json:"available"`
	ReplenishableScore *float64 `json:"replenishable_score"`
}

func (s *Service) UpdateProduct(ctx context.Context, id string, p ProductPatch) (domain.Product, error) {
	if p.ReplenishableScore != nil {
		v := *p.ReplenishableScore
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return domain.Product{}, invalid("replenishable_score must be between 0 and 1")
		}
	}
	return s.Store.UpdateProduct(ctx, id, p)
}
func (s *Service) Purchase(ctx context.Context, p domain.Purchase, key string) (domain.Purchase, error) {
	if !domain.ValidID(p.UserID) || p.PurchasedAt.IsZero() || p.PurchasedAt.After(s.now()) || len(p.Items) == 0 || len(p.Items) > 100 || p.TotalAmount < 0 || len(key) == 0 || len(key) > 200 {
		return p, invalid("purchase needs user, past timestamp, 1-100 items, nonnegative total and Idempotency-Key")
	}
	switch p.Source {
	case "app", "receipt", "POS", "imported_invoice":
	default:
		return p, invalid("unsupported purchase source")
	}
	seen := map[string]bool{}
	var total int64
	p.Items = append([]domain.PurchaseItem(nil), p.Items...)
	p.UserID = strings.ToLower(p.UserID)
	for i := range p.Items {
		p.Items[i].ProductID = strings.ToLower(p.Items[i].ProductID)
	}
	for _, item := range p.Items {
		if !domain.ValidID(item.ProductID) || seen[item.ProductID] || item.Quantity <= 0 || item.UnitPrice < 0 {
			return p, invalid("invalid or duplicate purchase item")
		}
		seen[item.ProductID] = true
		if item.UnitPrice > 0 && item.Quantity > (math.MaxInt64-total)/item.UnitPrice {
			return p, invalid("purchase total overflow")
		}
		total += item.Quantity * item.UnitPrice
	}
	if total != p.TotalAmount {
		return p, invalid("total_amount must equal quantity times unit_price summed across items")
	}
	p.ID = ""
	p.PurchasedAt = p.PurchasedAt.UTC()
	sort.Slice(p.Items, func(i, j int) bool { return p.Items[i].ProductID < p.Items[j].ProductID })
	encoded, _ := json.Marshal(p)
	sum := sha256.Sum256(encoded)
	hash := hex.EncodeToString(sum[:])
	err := s.Store.WithUser(ctx, p.UserID, func(tx Transaction) error {
		prior, priorHash, err := tx.PurchaseByKey(ctx, key)
		if err != nil {
			return err
		}
		if prior != nil {
			if priorHash != hash {
				return domain.ErrConflict
			}
			p = *prior
			return nil
		}
		f, err := tx.Facts(ctx)
		if err != nil {
			return err
		}
		p.ID = domain.NewID()
		if err = tx.SavePurchase(ctx, p, key, hash); err != nil {
			return err
		}
		for _, item := range p.Items {
			id := item.ProductID
			e := domain.Event{ID: domain.NewID(), UserID: p.UserID, Type: "PURCHASED", ProductID: &id, CreatedAt: p.PurchasedAt}
			e.Metadata, _ = json.Marshal(map[string]any{"purchase_id": p.ID, "quantity": item.Quantity})
			var latest *domain.Event
			for i := range f.Events {
				event := &f.Events[i]
				if event.Type == "SUGGESTION_SHOWN" && event.ProductID != nil && *event.ProductID == id && !event.CreatedAt.After(p.PurchasedAt) && p.PurchasedAt.Sub(event.CreatedAt) <= 7*24*time.Hour && (latest == nil || event.CreatedAt.After(latest.CreatedAt)) {
					latest = event
				}
			}
			if latest != nil {
				e.DecisionID = latest.DecisionID
				e.SuggestionID = latest.SuggestionID
			}
			if err = tx.SaveEvent(ctx, e); err != nil {
				return err
			}
			// Imported historical receipts must not clear a user's present shopping state.
			if s.now().Sub(p.PurchasedAt) <= 24*time.Hour {
				if err = tx.SaveMembership(ctx, id, domain.Membership{}); err != nil {
					return err
				}
			}
		}
		// A pending proposal is fulfilled only if every member was purchased after creation.
		for _, sg := range f.Suggestions {
			if sg.Status != "pending" {
				continue
			}
			all := true
			for _, item := range sg.Items {
				bought := seen[item.ProductID] && !p.PurchasedAt.Before(sg.CreatedAt)
				for _, o := range f.History[item.ProductID] {
					if !o.PurchasedAt.Before(sg.CreatedAt) {
						bought = true
					}
				}
				all = all && bought
			}
			if all {
				if err = tx.SetSuggestionStatus(ctx, sg.ID, "fulfilled"); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return p, err
}
func (s *Service) SetMembership(ctx context.Context, user, product string, m domain.Membership) error {
	return s.Store.WithUser(ctx, user, func(tx Transaction) error {
		f, err := tx.Facts(ctx)
		if err != nil {
			return err
		}
		old := f.Membership[product]
		if err = tx.SaveMembership(ctx, product, m); err != nil {
			return err
		}
		events := []string{}
		if m.InCart != old.InCart {
			if m.InCart {
				events = append(events, "ADDED_TO_CART")
			} else {
				events = append(events, "REMOVED_FROM_CART")
			}
		}
		if m.InList != old.InList {
			if m.InList {
				events = append(events, "ADDED_TO_LIST")
			} else {
				events = append(events, "REMOVED_FROM_LIST")
			}
		}
		for _, kind := range events {
			if err = tx.SaveEvent(ctx, domain.Event{ID: domain.NewID(), UserID: user, ProductID: &product, Type: kind, Metadata: json.RawMessage(`{}`), CreatedAt: s.now()}); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Service) Observe(ctx context.Context, user, product, kind string) error {
	// Trusted flows own purchase, membership and feedback events.
	if kind != "PRODUCT_VIEWED" {
		return invalid("only PRODUCT_VIEWED is accepted here; use purchase, state and feedback endpoints")
	}
	return s.Store.WithUser(ctx, user, func(tx Transaction) error {
		return tx.SaveEvent(ctx, domain.Event{ID: domain.NewID(), UserID: user, ProductID: &product, Type: kind, Metadata: json.RawMessage(`{}`), CreatedAt: s.now()})
	})
}
