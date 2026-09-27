package app

import (
	"context"
	"smartreplenish/internal/decision"
	"smartreplenish/internal/domain"
	"time"
)

type Rate struct {
	Numerator   int      `json:"numerator"`
	Denominator int      `json:"denominator"`
	Rate        *float64 `json:"rate"`
}

func ratio(n, d int) Rate {
	r := Rate{Numerator: n, Denominator: d}
	if d > 0 {
		v := float64(n) / float64(d)
		r.Rate = &v
	}
	return r
}

type EngineMetrics struct {
	Decisions        int      `json:"decisions"`
	Errors           int      `json:"errors"`
	AverageLatencyMS float64  `json:"average_latency_ms"`
	KnownCostUSD     *float64 `json:"known_cost_usd"`
	CostCoverage     int      `json:"cost_coverage"`
}

// RunningLowMetrics treat each answer as an explicit label for the prediction.
type RunningLowMetrics struct {
	QuestionsShown         int  `json:"questions_shown"`
	Confirmed              Rate `json:"confirmed_prediction_precision"`
	Denied                 Rate `json:"denied"`
	PurchaseAfterConfirm7d Rate `json:"purchase_within_7d_after_confirm"`
}
type Metrics struct {
	AsOf                time.Time                `json:"as_of"`
	Acceptance          Rate                     `json:"suggestion_acceptance"`
	Dismissal           Rate                     `json:"suggestion_dismissal_false_positive_proxy"`
	Conversion          map[string]Rate          `json:"purchase_conversion"`
	NoActionOpportunity Rate                     `json:"no_action_purchase_within_7d_proxy"`
	ShadowCoverage      Rate                     `json:"shadow_valid_pair_coverage"`
	ShadowAgreement     Rate                     `json:"shadow_action_agreement"`
	Engines             map[string]EngineMetrics `json:"engines"`
	RunningLow          RunningLowMetrics        `json:"running_low"`
	Notes               []string                 `json:"notes"`
}

func (s *Service) Metrics(ctx context.Context, user string) (Metrics, error) {
	f, err := s.Store.Facts(ctx, user)
	if err != nil {
		return Metrics{}, err
	}
	logs, err := s.Store.AllDecisions(ctx, user)
	if err != nil {
		return Metrics{}, err
	}
	return ComputeMetrics(f.Events, logs, s.now()), nil
}

// ComputeMetrics counts each decision once and excludes immature conversion windows.
func ComputeMetrics(events []domain.Event, logs []DecisionLog, now time.Time) Metrics {
	m := Metrics{AsOf: now, Conversion: map[string]Rate{}, Engines: map[string]EngineMetrics{}, Notes: []string{"Item-level descriptive rates, not causal lift; unknown rates and costs are null.", "Dismissal is a false-positive proxy. No outbound notifications are sent.", "No-action episodes are non-overlapping seven-day windows per product.", "Late receipts may revise metrics; conversion windows start at the first shown event.", "Running-low precision counts yes answers among shown questions; unanswered questions stay in the denominator."}}
	shown := map[string]time.Time{}
	accepted, dismissed := map[string]bool{}, map[string]bool{}
	purchases := map[string][]time.Time{}
	productPurchases := map[string][]time.Time{}
	for _, e := range events {
		if e.CreatedAt.After(now) {
			continue
		}
		if e.Type == "PURCHASED" && e.ProductID != nil {
			productPurchases[*e.ProductID] = append(productPurchases[*e.ProductID], e.CreatedAt)
		}
		if e.DecisionID == nil {
			continue
		}
		id := *e.DecisionID
		switch e.Type {
		case "SUGGESTION_SHOWN":
			if t, ok := shown[id]; !ok || e.CreatedAt.Before(t) {
				shown[id] = e.CreatedAt
			}
		case "SUGGESTION_ACCEPTED":
			accepted[id] = true
		case "SUGGESTION_DISMISSED":
			dismissed[id] = true
		case "PURCHASED":
			purchases[id] = append(purchases[id], e.CreatedAt)
		}
	}
	a, d := 0, 0
	for id := range shown {
		if accepted[id] {
			a++
		}
		if dismissed[id] {
			d++
		}
	}
	m.Acceptance = ratio(a, len(shown))
	m.Dismissal = ratio(d, len(shown))
	for name, window := range map[string]time.Duration{"24h": 24 * time.Hour, "3d": 72 * time.Hour, "7d": 168 * time.Hour} {
		n, total := 0, 0
		for id, start := range shown {
			if now.Sub(start) < window {
				continue
			}
			total++
			for _, p := range purchases[id] {
				if !p.Before(start) && p.Sub(start) <= window {
					n++
					break
				}
			}
		}
		m.Conversion[name] = ratio(n, total)
	}
	prod, shadow := map[string]DecisionLog{}, map[string]DecisionLog{}
	episodeEnd := map[string]time.Time{}
	opportunities, episodes := 0, 0
	for _, l := range logs {
		if l.CreatedAt.After(now) {
			continue
		}
		em := m.Engines[l.Engine]
		em.Decisions++
		em.AverageLatencyMS += float64(l.LatencyMS)
		if l.Error != nil {
			em.Errors++
		}
		if l.Result != nil && l.Result.CostUSD != nil {
			if em.KnownCostUSD == nil {
				v := 0.0
				em.KnownCostUSD = &v
			}
			*em.KnownCostUSD += *l.Result.CostUSD
			em.CostCoverage++
		}
		m.Engines[l.Engine] = em
		key := l.EvaluationID + ":" + l.ProductID
		if l.Shadow {
			shadow[key] = l
			continue
		}
		prod[key] = l
		if l.Error == nil && !l.ExecutedAction.CreatesSuggestion() && now.Sub(l.CreatedAt) >= 7*24*time.Hour && !l.CreatedAt.Before(episodeEnd[l.ProductID]) {
			episodes++
			episodeEnd[l.ProductID] = l.CreatedAt.Add(7 * 24 * time.Hour)
			for _, p := range productPurchases[l.ProductID] {
				if p.After(l.CreatedAt) && !p.After(episodeEnd[l.ProductID]) {
					opportunities++
					break
				}
			}
		}
	}
	pairs, agree := 0, 0
	for key, p := range prod {
		sh, ok := shadow[key]
		if ok && p.Error == nil && sh.Error == nil && p.Result != nil && sh.Result != nil {
			pairs++
			if p.Result.Action == sh.Result.Action {
				agree++
			}
		}
	}
	asked, yes, no, mature, bought := 0, 0, 0, 0, 0
	for _, p := range prod {
		start, ok := shown[p.ID]
		if !ok || p.ExecutedAction != decision.AskIfRunningLow {
			continue
		}
		asked++
		if dismissed[p.ID] {
			no++
		}
		if !accepted[p.ID] {
			continue
		}
		yes++
		if now.Sub(start) < 7*24*time.Hour {
			continue
		}
		mature++
		for _, t := range purchases[p.ID] {
			if !t.Before(start) && t.Sub(start) <= 7*24*time.Hour {
				bought++
				break
			}
		}
	}
	m.RunningLow = RunningLowMetrics{QuestionsShown: asked, Confirmed: ratio(yes, asked), Denied: ratio(no, asked), PurchaseAfterConfirm7d: ratio(bought, mature)}
	m.ShadowCoverage = ratio(pairs, len(prod))
	m.ShadowAgreement = ratio(agree, pairs)
	m.NoActionOpportunity = ratio(opportunities, episodes)
	for k, v := range m.Engines {
		v.AverageLatencyMS /= float64(v.Decisions)
		m.Engines[k] = v
	}
	return m
}
