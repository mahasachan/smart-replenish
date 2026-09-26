package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"smartreplenish/internal/app"
	"smartreplenish/internal/domain"
	"strconv"
	"strings"
)

type API struct{ Service *app.Service }
type endpoint func(http.ResponseWriter, *http.Request) (any, error)

func (a API) Handler() http.Handler {
	mux := http.NewServeMux()
	handle := func(pattern string, status int, fn endpoint) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			for _, name := range []string{"userId", "productId", "suggestionId"} {
				if id := r.PathValue(name); id != "" && !domain.ValidID(id) {
					respond(w, http.StatusBadRequest, map[string]string{"error": "invalid UUID"})
					return
				}
				r.SetPathValue(name, strings.ToLower(r.PathValue(name)))
			}
			value, err := fn(w, r)
			if err != nil {
				code, message := http.StatusInternalServerError, "internal server error"
				switch {
				case errors.Is(err, domain.ErrInvalid):
					code, message = http.StatusBadRequest, err.Error()
				case errors.Is(err, domain.ErrNotFound):
					code, message = http.StatusNotFound, "not found"
				case errors.Is(err, domain.ErrConflict):
					code, message = http.StatusConflict, "conflicting request or feedback"
				default:
					slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
				}
				respond(w, code, map[string]string{"error": message})
				return
			}
			respond(w, status, value)
		})
	}
	handle("GET /healthz", 200, func(http.ResponseWriter, *http.Request) (any, error) { return map[string]string{"status": "ok"}, nil })
	handle("POST /users", 201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			Timezone            string `json:"timezone"`
			NotificationEnabled bool   `json:"notification_enabled"`
			SuggestionEnabled   *bool  `json:"suggestion_enabled"`
		}
		if err := decode(w, r, &in); err != nil {
			return nil, err
		}
		enabled := true
		if in.SuggestionEnabled != nil {
			enabled = *in.SuggestionEnabled
		}
		return a.Service.CreateUser(r.Context(), domain.User{Timezone: in.Timezone, NotificationEnabled: in.NotificationEnabled, SuggestionEnabled: enabled})
	})
	handle("PATCH /users/{userId}", 200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var p app.UserPatch
		if err := decode(w, r, &p); err != nil {
			return nil, err
		}
		return a.Service.UpdateUser(r.Context(), r.PathValue("userId"), p)
	})
	handle("POST /products", 201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			SKU                string  `json:"sku"`
			Name               string  `json:"name"`
			Category           string  `json:"category"`
			Brand              string  `json:"brand"`
			Unit               string  `json:"unit"`
			ReplenishableScore float64 `json:"replenishable_score"`
			Available          *bool   `json:"available"`
		}
		if err := decode(w, r, &in); err != nil {
			return nil, err
		}
		available := true
		if in.Available != nil {
			available = *in.Available
		}
		return a.Service.CreateProduct(r.Context(), domain.Product{SKU: in.SKU, Name: in.Name, Category: in.Category, Brand: in.Brand, Unit: in.Unit, ReplenishableScore: in.ReplenishableScore, Available: available})
	})
	handle("PATCH /products/{productId}", 200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var p app.ProductPatch
		if err := decode(w, r, &p); err != nil {
			return nil, err
		}
		return a.Service.UpdateProduct(r.Context(), r.PathValue("productId"), p)
	})
	handle("POST /purchases", 201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var p domain.Purchase
		if err := decode(w, r, &p); err != nil {
			return nil, err
		}
		if p.ID != "" {
			return nil, domain.ErrInvalid
		}
		return a.Service.Purchase(r.Context(), p, r.Header.Get("Idempotency-Key"))
	})
	handle("PUT /users/{userId}/products/{productId}/state", 200, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var m domain.Membership
		if err := decode(w, r, &m); err != nil {
			return nil, err
		}
		err := a.Service.SetMembership(r.Context(), r.PathValue("userId"), r.PathValue("productId"), m)
		return m, err
	})
	handle("POST /users/{userId}/events", 201, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var e struct {
			ProductID string `json:"product_id"`
			Type      string `json:"event_type"`
		}
		if err := decode(w, r, &e); err != nil {
			return nil, err
		}
		if !domain.ValidID(e.ProductID) {
			return nil, domain.ErrInvalid
		}
		return map[string]bool{"recorded": true}, a.Service.Observe(r.Context(), r.PathValue("userId"), e.ProductID, e.Type)
	})
	handle("GET /users/{userId}/replenishment", 200, func(_ http.ResponseWriter, r *http.Request) (any, error) {
		return a.Service.Replenishment(r.Context(), r.PathValue("userId"))
	})
	handle("POST /users/{userId}/decisions/evaluate", 200, func(_ http.ResponseWriter, r *http.Request) (any, error) {
		return a.Service.Evaluate(r.Context(), r.PathValue("userId"))
	})
	handle("GET /users/{userId}/suggestions", 200, func(_ http.ResponseWriter, r *http.Request) (any, error) {
		limit, offset, err := page(r)
		if err != nil {
			return nil, err
		}
		f, err := a.Service.Store.Facts(r.Context(), r.PathValue("userId"))
		if err != nil {
			return nil, err
		}
		items := []domain.Suggestion{}
		for i := len(f.Suggestions) - 1 - offset; i >= 0 && len(items) < limit; i-- {
			items = append(items, f.Suggestions[i])
		}
		return items, nil
	})
	for _, verb := range []string{"shown", "accept", "dismiss"} {
		handle("POST /suggestions/{suggestionId}/"+verb, 200, func(_ http.ResponseWriter, r *http.Request) (any, error) {
			return a.Service.Feedback(r.Context(), r.PathValue("suggestionId"), verb)
		})
	}
	handle("GET /users/{userId}/decision-history", 200, func(_ http.ResponseWriter, r *http.Request) (any, error) {
		limit, offset, err := page(r)
		if err != nil {
			return nil, err
		}
		if _, err = a.Service.Store.Facts(r.Context(), r.PathValue("userId")); err != nil {
			return nil, err
		}
		return a.Service.Store.History(r.Context(), r.PathValue("userId"), limit, offset)
	})
	handle("GET /users/{userId}/evaluation-metrics", 200, func(_ http.ResponseWriter, r *http.Request) (any, error) {
		return a.Service.Metrics(r.Context(), r.PathValue("userId"))
	})
	return mux
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	var raw json.RawMessage
	if err := d.Decode(&raw); err != nil {
		return domain.ErrInvalid
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return domain.ErrInvalid
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return domain.ErrInvalid
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return domain.ErrInvalid
	}
	return nil
}
func respond(w http.ResponseWriter, status int, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		status = http.StatusInternalServerError
		raw = []byte(`{"error":"response encoding failed"}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(raw, '\n'))
}
func page(r *http.Request) (int, int, error) {
	limit, offset := 50, 0
	var err error
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil {
			return 0, 0, domain.ErrInvalid
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		offset, err = strconv.Atoi(v)
		if err != nil {
			return 0, 0, domain.ErrInvalid
		}
	}
	if limit < 1 || limit > 100 || offset < 0 || offset > 100000 {
		return 0, 0, domain.ErrInvalid
	}
	return limit, offset, nil
}
