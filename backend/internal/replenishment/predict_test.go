package replenishment

import (
	"math"
	"smartreplenish/internal/domain"
	"testing"
	"time"
)

func history(intervals ...int) []domain.Observation {
	t := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	out := []domain.Observation{{PurchasedAt: t, Quantity: 1}}
	for _, d := range intervals {
		t = t.AddDate(0, 0, d)
		out = append(out, domain.Observation{PurchasedAt: t, Quantity: 1})
	}
	return out
}
func TestMedian(t *testing.T) {
	for _, tc := range []struct {
		v    []float64
		want float64
	}{{nil, 0}, {[]float64{7}, 7}, {[]float64{8, 7}, 7.5}, {[]float64{7, 8, 7, 31, 8}, 8}} {
		if got := Median(tc.v); got != tc.want {
			t.Fatalf("median %v = %v, want %v", tc.v, got, tc.want)
		}
	}
}
func TestPredictionConfidenceAndOutlier(t *testing.T) {
	product := domain.Product{ID: "milk", ReplenishableScore: .98}
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	regular := Predict(product, history(7, 7, 7, 7), now, time.UTC)
	irregular := Predict(product, history(7, 8, 7, 31, 8), now, time.UTC)
	sparse := Predict(product, history(7, 7), now, time.UTC)
	if regular.PredictionConfidence != .98 || regular.PurchaseRegularity != 1 {
		t.Fatalf("regular: %+v", regular)
	}
	if irregular.MedianRepurchaseDays != 8 || irregular.PredictionConfidence >= regular.PredictionConfidence {
		t.Fatalf("outlier: %+v", irregular)
	}
	if sparse.PredictionConfidence >= regular.PredictionConfidence {
		t.Fatal("sample count should lower confidence")
	}
	product.ReplenishableScore = .6
	lower := Predict(product, history(7, 7, 7, 7), now, time.UTC)
	if lower.PredictionConfidence >= regular.PredictionConfidence {
		t.Fatal("product score should lower confidence")
	}
	if *regular.EstimatedDaysRemaining >= 0 {
		t.Fatal("overdue must be negative")
	}
}
func TestEligibilityAndSameDay(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := domain.Product{ReplenishableScore: .98}
	for _, tc := range []struct {
		score  float64
		obs    []domain.Observation
		reason string
	}{{.01, history(7, 7, 7), "low_replenishability"}, {.98, history(7), "insufficient_history"}} {
		p.ReplenishableScore = tc.score
		got := Predict(p, tc.obs, now, time.UTC)
		if got.Eligible || got.EstimatedDaysRemaining != nil || got.Reason != tc.reason {
			t.Fatalf("unexpected %+v", got)
		}
	}
	p.ReplenishableScore = .98
	obs := history(7, 7)
	obs = append(obs, domain.Observation{PurchasedAt: obs[0].PurchasedAt.Add(time.Hour), Quantity: 3}, domain.Observation{PurchasedAt: now.Add(time.Hour), Quantity: 100})
	got := Predict(p, obs, now, time.UTC)
	if got.TotalPurchaseCount != 3 || math.Abs(got.AverageQuantity-2) > 1e-9 {
		t.Fatalf("same day / future: %+v", got)
	}
}
func TestCalendarDaysAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	first := time.Date(2026, 3, 1, 12, 0, 0, 0, loc)
	obs := []domain.Observation{}
	for i := 0; i < 4; i++ {
		obs = append(obs, domain.Observation{PurchasedAt: first.AddDate(0, 0, 7*i), Quantity: 1})
	}
	p := Predict(domain.Product{ReplenishableScore: 1}, obs, first.AddDate(0, 0, 23), loc)
	if p.MedianRepurchaseDays != 7 || p.IntervalVariance != 0 {
		t.Fatalf("DST changed calendar interval: %+v", p)
	}
}
func TestMedianQuantityPerOccasion(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	day := func(d, q int) domain.Observation {
		return domain.Observation{PurchasedAt: time.Date(2026, 2, d, 9, 0, 0, 0, time.UTC), Quantity: int64(q)}
	}
	// Two receipts on Feb 8 form one occasion of three units.
	p := Predict(domain.Product{ReplenishableScore: .9}, []domain.Observation{day(1, 2), day(8, 1), day(8, 2), day(15, 2), day(22, 6)}, now, time.UTC)
	if p.MedianQuantity != 2.5 {
		t.Fatalf("median quantity %v", p.MedianQuantity)
	}
	if Predict(domain.Product{}, nil, now, time.UTC).MedianQuantity != 0 {
		t.Fatal("empty history must have zero median quantity")
	}
}
