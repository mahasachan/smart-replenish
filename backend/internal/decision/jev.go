package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// JevDecisionEngine implements the verified TypeSafe v1 HTTP Choice contract.
// https://docs.typesafe.ai/api.md (verified 2026-09-21).
type JevDecisionEngine struct {
	APIKey, Model, Endpoint string
	Client                  *http.Client
}

func (j JevDecisionEngine) Decide(ctx context.Context, c DecisionContext) (DecisionResult, error) {
	start := time.Now()
	model := j.Model
	if model == "" {
		model = "jev-latest"
	}
	result := DecisionResult{Provider: "jev", Model: model, ContextVersion: c.Version}
	fail := func(err error) (DecisionResult, error) {
		result.LatencyMS = time.Since(start).Milliseconds()
		return result, err
	}
	if j.APIKey == "" {
		return fail(errors.New("jev is not configured"))
	}
	criteria := map[Action]string{
		DoNothing:                "Do not make a suggestion because it is not useful.",
		Wait:                     "Reconsider later; no suggestion now.",
		SuggestNow:               "Offer this product now because it is likely needed soon.",
		SuggestBundle:            "Nominate this product for a replenishment bundle; application code chooses eligible members.",
		AddToSmartListSuggestion: "Propose this product in the suggested list section, without adding to the actual list.",
		AskIfRunningLow:          "Ask whether this product is running low because the need is uncertain.",
	}
	request := map[string]any{"state": c, "model": model, "questions": map[string]any{"next_action": map[string]any{"type": "choice", "instructions": "Choose the most useful next replenishment action for `product` given `user` and `shopping_context`. Prediction confidence is uncertain evidence, not certainty. Consider timing, recent suggestions, feedback, and whether the user is shopping. Choose only the supplied action; application policy controls execution.", "criteria": criteria}}}
	body, err := json.Marshal(request)
	if err != nil {
		return fail(err)
	}
	endpoint := j.Endpoint
	if endpoint == "" {
		endpoint = "https://api.typesafe.ai/v1/systemone"
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fail(err)
	}
	req.Header.Set("Authorization", "Bearer "+j.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client := j.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fail(errors.New("jev request failed or timed out"))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fail(fmt.Errorf("jev returned HTTP %d", resp.StatusCode))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return fail(errors.New("invalid jev response size"))
	}
	var payload struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Type          string             `json:"type"`
			Choice        Action             `json:"choice"`
			Confidence    *float64           `json:"confidence"`
			Probabilities map[Action]float64 `json:"probabilities"`
		} `json:"answers"`
		Usage struct {
			Cost         *float64 `json:"cost"`
			InputTokens  int      `json:"input_tokens"`
			OutputTokens int      `json:"output_tokens"`
		} `json:"usage"`
	}
	if err = json.Unmarshal(raw, &payload); err != nil {
		return fail(errors.New("malformed jev response"))
	}
	a, ok := payload.Answers["next_action"]
	if !ok || a.Type != "choice" || !a.Choice.Valid() || a.Confidence == nil || !unit(*a.Confidence) || payload.Model == "" || len(a.Probabilities) != len(criteria) {
		return fail(errors.New("invalid jev choice response"))
	}
	sum := 0.0
	selected, exists := a.Probabilities[a.Choice]
	if !exists {
		return fail(errors.New("missing selected probability"))
	}
	for option, p := range a.Probabilities {
		if !option.Valid() || !unit(p) || p > selected+1e-9 {
			return fail(errors.New("invalid jev probability distribution"))
		}
		sum += p
	}
	if math.Abs(sum-1) > 0.001 {
		return fail(errors.New("jev probabilities do not sum to one"))
	}
	result.Action = a.Choice
	result.Confidence = *a.Confidence
	result.Probability = &selected
	result.Model = payload.Model
	result.CostUSD = payload.Usage.Cost
	// Preserve only documented structured metadata, never arbitrary provider prose.
	result.Metadata, _ = json.Marshal(map[string]any{"probabilities": a.Probabilities, "usage": payload.Usage})
	result.LatencyMS = time.Since(start).Milliseconds()
	return result, nil
}
func unit(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }
