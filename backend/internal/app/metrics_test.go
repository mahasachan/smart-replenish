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
