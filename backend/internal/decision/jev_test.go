package decision

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const validResponse = `{"model":"jev-test","answers":{"next_action":{"type":"choice","choice":"SUGGEST_NOW","confidence":0.8,"probabilities":{"DO_NOTHING":0.02,"WAIT":0.03,"SUGGEST_NOW":0.85,"SUGGEST_BUNDLE":0.04,"ADD_TO_SMART_LIST_SUGGESTION":0.03,"ASK_IF_RUNNING_LOW":0.03}}},"usage":{"input_tokens":100,"output_tokens":25}}`

func TestJevContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("incorrect authentication/method")
		}
		var body struct {
			State     DecisionContext `json:"state"`
			Questions map[string]struct {
				Type     string            `json:"type"`
				Criteria map[string]string `json:"criteria"`
			} `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.State.Version != "v1" || body.Questions["next_action"].Type != "choice" || len(body.Questions["next_action"].Criteria) != 6 {
			t.Error("invalid bounded request")
		}
		_, _ = w.Write([]byte(validResponse))
	}))
	defer server.Close()
	r, err := (JevDecisionEngine{APIKey: "test-key", Endpoint: server.URL}).Decide(context.Background(), DecisionContext{Version: "v1"})
	if err != nil || r.Action != SuggestNow || r.Confidence != .8 || *r.Probability != .85 || r.CostUSD != nil || r.Model != "jev-test" {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestJevRejectsMalformedAndUnavailable(t *testing.T) {
	for name, response := range map[string]string{"unknown action": strings.Replace(validResponse, `"choice":"SUGGEST_NOW"`, `"choice":"BUY_NOW"`, 1), "missing confidence": strings.Replace(validResponse, `"confidence":0.8,`, "", 1), "invalid sum": strings.Replace(validResponse, `"SUGGEST_NOW":0.85`, `"SUGGEST_NOW":0.95`, 1), "missing answer": `{"answers":{}}`, "garbage": "not-json"} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(response)) }))
			defer server.Close()
			if _, err := (JevDecisionEngine{APIKey: "test", Endpoint: server.URL}).Decide(context.Background(), DecisionContext{}); err == nil {
				t.Fatal("accepted invalid response")
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }))
	defer server.Close()
	if _, err := (JevDecisionEngine{APIKey: "test", Endpoint: server.URL}).Decide(context.Background(), DecisionContext{}); err == nil {
		t.Fatal("accepted rate limit")
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := (JevDecisionEngine{APIKey: "test", Endpoint: server.URL}).Decide(ctx, DecisionContext{}); err == nil {
		t.Fatal("ignored cancellation")
	}
}
