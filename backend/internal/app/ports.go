package app

import (
	"context"
	"smartreplenish/internal/decision"
	"smartreplenish/internal/domain"
	"smartreplenish/internal/policy"
	"time"
)

type Facts struct {
	User          domain.User
	Products      map[string]domain.Product
	History       map[string][]domain.Observation
	Membership    map[string]domain.Membership
	PurchaseDates []time.Time
	Suggestions   []domain.Suggestion
	Events        []domain.Event
}
type DecisionLog struct {
	ID             string                   `json:"id"`
	EvaluationID   string                   `json:"evaluation_id"`
	UserID         string                   `json:"user_id"`
	ProductID      string                   `json:"product_id"`
	Engine         string                   `json:"engine"`
	Shadow         bool                     `json:"shadow"`
	Context        decision.DecisionContext `json:"decision_context"`
	Result         *decision.DecisionResult `json:"decision_result"`
	Policy         *policy.Result           `json:"policy_result"`
	ExecutedAction decision.Action          `json:"executed_action"`
	Error          *string                  `json:"error,omitempty"`
	LatencyMS      int64                    `json:"latency_ms"`
	CreatedAt      time.Time                `json:"created_at"`
}

// Transaction is scoped to one locked user. Implementations contain persistence only.
type Transaction interface {
	Facts(context.Context) (Facts, error)
	SaveUser(context.Context, domain.User) error
	PurchaseByKey(context.Context, string) (*domain.Purchase, string, error)
	SavePurchase(context.Context, domain.Purchase, string, string) error
	SaveMembership(context.Context, string, domain.Membership) error
	SaveEvent(context.Context, domain.Event) error
	SaveDecision(context.Context, DecisionLog) error
	SaveSuggestion(context.Context, domain.Suggestion) error
	SetSuggestionStatus(context.Context, string, string) error
}
type Store interface {
	CreateUser(context.Context, domain.User) error
	CreateProduct(context.Context, domain.Product) error
	UpdateProduct(context.Context, string, ProductPatch) (domain.Product, error)
	Products(context.Context, int, int) ([]domain.Product, error)
	Facts(context.Context, string) (Facts, error)
	WithUser(context.Context, string, func(Transaction) error) error
	Suggestion(context.Context, string) (domain.Suggestion, error)
	History(context.Context, string, int, int) ([]DecisionLog, error)
	AllDecisions(context.Context, string) ([]DecisionLog, error)
	SaveShadow(context.Context, DecisionLog) error
}
type Service struct {
	Store      Store
	Production decision.DecisionEngine
	Shadow     decision.DecisionEngine
	Now        func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
