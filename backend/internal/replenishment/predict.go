package replenishment

import (
	"math"
	"smartreplenish/internal/domain"
	"sort"
	"time"
)

type Prediction struct {
	ProductID              string    `json:"product_id"`
	Eligible               bool      `json:"eligible"`
	Reason                 string    `json:"reason,omitempty"`
	TotalPurchaseCount     int       `json:"total_purchase_count"`
	LastPurchaseDate       time.Time `json:"last_purchase_date"`
	DaysSinceLastPurchase  float64   `json:"days_since_last_purchase"`
	MedianRepurchaseDays   float64   `json:"median_repurchase_days"`
	MeanRepurchaseDays     float64   `json:"mean_repurchase_days"`
	IntervalVariance       float64   `json:"interval_variance"`
	PurchaseRegularity     float64   `json:"purchase_regularity"`
	AverageQuantity        float64   `json:"average_quantity"`
	EstimatedDaysRemaining *float64  `json:"estimated_days_remaining"`
	PredictionConfidence   float64   `json:"prediction_confidence"`
}

func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	n := len(v)
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}

// Calendar dates avoid artificial 23/25-hour intervals across daylight-saving changes.
func day(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
func Predict(product domain.Product, observations []domain.Observation, now time.Time, loc *time.Location) Prediction {
	p := Prediction{ProductID: product.ID}
	quantities := map[time.Time]float64{}
	for _, o := range observations {
		if o.PurchasedAt.After(now) || o.Quantity <= 0 {
			continue
		}
		quantities[day(o.PurchasedAt, loc)] += float64(o.Quantity)
		if o.PurchasedAt.After(p.LastPurchaseDate) {
			p.LastPurchaseDate = o.PurchasedAt
		}
	}
	days := make([]time.Time, 0, len(quantities))
	for d, q := range quantities {
		days = append(days, d)
		p.AverageQuantity += q
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
	p.TotalPurchaseCount = len(days)
	if len(days) > 0 {
		p.AverageQuantity /= float64(len(days))
		p.DaysSinceLastPurchase = now.Sub(p.LastPurchaseDate).Hours() / 24
	}
	if product.ReplenishableScore < 0.5 {
		p.Reason = "low_replenishability"
		return p
	}
	if len(days) < 3 {
		p.Reason = "insufficient_history"
		return p
	}
	intervals := make([]float64, 0, len(days)-1)
	for i := 1; i < len(days); i++ {
		intervals = append(intervals, days[i].Sub(days[i-1]).Hours()/24)
	}
	p.MedianRepurchaseDays = Median(intervals)
	for _, v := range intervals {
		p.MeanRepurchaseDays += v
	}
	p.MeanRepurchaseDays /= float64(len(intervals))
	var deviation float64
	for _, v := range intervals {
		p.IntervalVariance += math.Pow(v-p.MeanRepurchaseDays, 2)
		deviation += math.Abs(v - p.MedianRepurchaseDays)
	}
	p.IntervalVariance /= float64(len(intervals))
	deviation /= float64(len(intervals))
	p.PurchaseRegularity = 1 / (1 + deviation/p.MedianRepurchaseDays)
	p.PredictionConfidence = math.Min(1, float64(len(intervals))/4) * p.PurchaseRegularity * math.Max(0, math.Min(1, product.ReplenishableScore))
	remaining := p.MedianRepurchaseDays - p.DaysSinceLastPurchase
	p.EstimatedDaysRemaining = &remaining
	p.Eligible = true
	return p
}
