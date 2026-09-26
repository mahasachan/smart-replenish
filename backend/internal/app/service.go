package app

import (
	"context"
	"fmt"
	"smartreplenish/internal/action"
	"smartreplenish/internal/decision"
	"smartreplenish/internal/domain"
	"smartreplenish/internal/policy"
	"smartreplenish/internal/replenishment"
	"time"
)

type Evaluation struct {
	ID            string              `json:"evaluation_id"`
	Decisions     []DecisionLog       `json:"decisions"`
	Suggestions   []domain.Suggestion `json:"suggestions"`
	ShadowEnabled bool                `json:"shadow_enabled"`
	ShadowWarning string              `json:"shadow_warning,omitempty"`
}

func (s *Service) Replenishment(ctx context.Context, user string) ([]replenishment.Prediction, error) {
	f, err := s.Store.Facts(ctx, user)
	if err != nil {
		return nil, err
	}
	out := []replenishment.Prediction{}
	for _, c := range contexts(f, s.now()) {
		out = append(out, c.Product.Prediction)
	}
	return out, nil
}
func (s *Service) Evaluate(ctx context.Context, user string) (Evaluation, error) {
	out := Evaluation{ID: domain.NewID(), Decisions: []DecisionLog{}, Suggestions: []domain.Suggestion{}, ShadowEnabled: s.Shadow != nil}
	engine := s.Production
	if engine == nil {
		engine = decision.RuleBasedDecisionEngine{}
	}
	// Production rules are local and run under the user lock; Jev always runs outside it.
	err := s.Store.WithUser(ctx, user, func(tx Transaction) error {
		f, err := tx.Facts(ctx)
		if err != nil {
			return err
		}
		now := s.now()
		cs := contexts(f, now)
		if len(cs) > 200 {
			return fmt.Errorf("%w: evaluation supports at most 200 purchased products", domain.ErrInvalid)
		}
		bundle := []action.Item{}
		bundleLogs := []int{}
		for _, c := range cs {
			start := time.Now()
			r, engineErr := engine.Decide(ctx, c)
			log := DecisionLog{ID: domain.NewID(), EvaluationID: out.ID, UserID: user, ProductID: c.Product.ProductID, Engine: "rules", Context: c, ExecutedAction: decision.DoNothing, CreatedAt: now, LatencyMS: time.Since(start).Milliseconds()}
			if engineErr != nil {
				message := "production decision failed"
				log.Error = &message
			} else {
				log.Result = &r
				p := (policy.PolicyEngine{}).Validate(r, c)
				p.ExecutionContext = &c
				log.Policy = &p
				log.ExecutedAction = p.Action
				if p.Allowed && p.Action.CreatesSuggestion() {
					item := action.Item{Product: f.Products[c.Product.ProductID], DecisionID: log.ID}
					if p.Action == decision.SuggestBundle {
						bundle = append(bundle, item)
						bundleLogs = append(bundleLogs, len(out.Decisions))
					} else {
						sg, executed := action.Build(user, p.Action, []action.Item{item}, now)
						log.ExecutedAction = executed
						out.Suggestions = append(out.Suggestions, *sg)
					}
				}
			}
			out.Decisions = append(out.Decisions, log)
		}
		if len(bundle) > 0 {
			sg, executed := action.Build(user, decision.SuggestBundle, bundle, now)
			out.Suggestions = append(out.Suggestions, *sg)
			for _, i := range bundleLogs {
				out.Decisions[i].ExecutedAction = executed
			}
		}
		for _, log := range out.Decisions {
			if err = tx.SaveDecision(ctx, log); err != nil {
				return err
			}
		}
		for _, sg := range out.Suggestions {
			if err = tx.SaveSuggestion(ctx, sg); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Evaluation{}, err
	}
	if s.Shadow == nil {
		return out, nil
	}
	// Preserve successful production even on caller cancellation; bounded cleanup is synchronous.
	shadowCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
	defer cancel()
	production := append([]DecisionLog(nil), out.Decisions...)
	for _, prod := range production {
		start := time.Now()
		r, err := s.Shadow.Decide(shadowCtx, prod.Context)
		log := DecisionLog{ID: domain.NewID(), EvaluationID: out.ID, UserID: user, ProductID: prod.ProductID, Engine: "jev", Shadow: true, Context: prod.Context, ExecutedAction: decision.DoNothing, CreatedAt: s.now(), LatencyMS: time.Since(start).Milliseconds()}
		if err != nil {
			message := err.Error()
			log.Error = &message
		} else {
			log.Result = &r
			p := (policy.PolicyEngine{}).Validate(r, prod.Context)
			log.Policy = &p
		}
		// Use a separate short persistence deadline when the provider budget is exhausted.
		saveCtx, saveCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		saveErr := s.Store.SaveShadow(saveCtx, log)
		saveCancel()
		if saveErr != nil {
			out.ShadowWarning = "production committed; some shadow logs could not be saved"
			break
		}
		out.Decisions = append(out.Decisions, log)
		if shadowCtx.Err() != nil {
			out.ShadowWarning = "production committed; shadow budget exhausted; remaining comparisons omitted"
			break
		}
	}
	return out, nil
}
