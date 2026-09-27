package app

import (
	"smartreplenish/internal/decision"
	"smartreplenish/internal/domain"
	"testing"
	"time"
)

func TestMetricsMatureWindowsAndDeduplication(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	old, recent, product := "old", "recent", "milk"
	start := now.Add(-8 * 24 * time.Hour)
	events := []domain.Event{{Type: "SUGGESTION_SHOWN", DecisionID: &old, CreatedAt: start}, {Type: "SUGGESTION_SHOWN", DecisionID: &recent, CreatedAt: now.Add(-time.Hour)}, {Type: "SUGGESTION_ACCEPTED", DecisionID: &old, CreatedAt: start.Add(time.Hour)}, {Type: "SUGGESTION_ACCEPTED", DecisionID: &old, CreatedAt: start.Add(time.Hour)}, {Type: "PURCHASED", DecisionID: &old, ProductID: &product, CreatedAt: start.Add(2 * time.Hour)}, {Type: "PURCHASED", DecisionID: &old, ProductID: &product, CreatedAt: start.Add(3 * time.Hour)}}
	result := &decision.DecisionResult{Action: decision.SuggestNow}
	logs := []DecisionLog{{ID: old, EvaluationID: "pair", ProductID: product, Engine: "rules", Result: result, ExecutedAction: decision.SuggestNow, CreatedAt: start}, {ID: "shadow", EvaluationID: "pair", ProductID: product, Engine: "jev", Shadow: true, Result: result, CreatedAt: start}}
	m := ComputeMetrics(events, logs, now)
	if m.Acceptance.Numerator != 1 || m.Acceptance.Denominator != 2 || m.Conversion["24h"].Denominator != 1 || m.Conversion["7d"].Numerator != 1 || m.ShadowAgreement.Numerator != 1 || m.Engines["jev"].KnownCostUSD != nil {
		t.Fatalf("%+v", m)
	}
	empty := ComputeMetrics(nil, nil, now)
	if empty.Acceptance.Rate != nil || empty.Conversion["7d"].Rate != nil {
		t.Fatal("unknown rate became zero")
	}
}
func TestRunningLowMetrics(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	start := now.Add(-8 * 24 * time.Hour)
	yes, no, unanswered, list, product := "yes", "no", "open", "list", "milk"
	ev := func(kind, id string, at time.Time) domain.Event {
		return domain.Event{Type: kind, DecisionID: &id, ProductID: &product, CreatedAt: at}
	}
	events := []domain.Event{ev("SUGGESTION_SHOWN", yes, start), ev("SUGGESTION_ACCEPTED", yes, start), ev("PURCHASED", yes, start.Add(48*time.Hour)),
		ev("SUGGESTION_SHOWN", no, now.Add(-time.Hour)), ev("SUGGESTION_DISMISSED", no, now.Add(-time.Hour)),
		ev("SUGGESTION_SHOWN", unanswered, now.Add(-time.Hour)),
		ev("SUGGESTION_SHOWN", list, start), ev("SUGGESTION_ACCEPTED", list, start)}
	log := func(id string, a decision.Action) DecisionLog {
		return DecisionLog{ID: id, EvaluationID: id, ProductID: product, Engine: "rules", ExecutedAction: a, CreatedAt: start}
	}
	logs := []DecisionLog{log(yes, decision.AskIfRunningLow), log(no, decision.AskIfRunningLow), log(unanswered, decision.AskIfRunningLow), log(list, decision.SuggestNow)}
	r := ComputeMetrics(events, logs, now).RunningLow
	if r.QuestionsShown != 3 || r.Confirmed.Numerator != 1 || r.Denied.Numerator != 1 || r.Denied.Denominator != 3 || r.PurchaseAfterConfirm7d.Numerator != 1 || r.PurchaseAfterConfirm7d.Denominator != 1 {
		t.Fatalf("%+v", r)
	}
}
func TestRunningLowSnoozeAndCartQuantity(t *testing.T) {
	if RunningLowSnooze(1) != 24*time.Hour || RunningLowSnooze(8) != 96*time.Hour {
		t.Fatal("snooze must be half the median interval, at least one day")
	}
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	obs := func(q ...int64) []domain.Observation {
		out := []domain.Observation{}
		for i, v := range q {
			out = append(out, domain.Observation{PurchasedAt: now.AddDate(0, 0, -7*(i+1)), Quantity: v})
		}
		return out
	}
	for _, tc := range []struct {
		history []domain.Observation
		want    int64
	}{{nil, 1}, {obs(1, 3, 3), 3}, {obs(2, 3), 3}, {obs(5000), domain.MaxCartQuantity}} {
		if got := CartQuantity(tc.history, now, time.UTC); got != tc.want {
			t.Fatalf("quantity %d, want %d", got, tc.want)
		}
	}
}
