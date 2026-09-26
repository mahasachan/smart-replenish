package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"os"
	"smartreplenish/internal/app"
	"smartreplenish/internal/decision"
	"smartreplenish/internal/domain"
	"smartreplenish/internal/httpapi"
	"smartreplenish/internal/postgres"
	"smartreplenish/migrations"
	"strings"
	"sync"
	"testing"
	"time"
)

func setup(t *testing.T) (*app.Service, *postgres.Store) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run isolated PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + strings.ReplaceAll(domain.NewID(), "-", "")
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, err := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		if err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal("migration replay", err)
	}
	store := &postgres.Store{Pool: pool}
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	return &app.Service{Store: store, Production: decision.RuleBasedDecisionEngine{}, Now: func() time.Time { return now }}, store
}
func seed(t *testing.T, s *app.Service) (domain.User, []domain.Product) {
	t.Helper()
	ctx := context.Background()
	u, err := s.CreateUser(ctx, domain.User{Timezone: "Asia/Bangkok", SuggestionEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	products := []domain.Product{{SKU: "milk", Name: "Milk", Category: "milk", Unit: "carton", ReplenishableScore: .98, Available: true}, {SKU: "eggs", Name: "Eggs", Category: "eggs", Unit: "box", ReplenishableScore: .95, Available: true}, {SKU: "tv", Name: "Television", Category: "electronics", Unit: "piece", ReplenishableScore: .01, Available: true}}
	for i := range products {
		products[i], err = s.CreateProduct(ctx, products[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, days := range [][]int{{1, 8, 15, 22, 30}, {1, 9, 17, 25}, {1}} {
		for _, day := range days {
			p := domain.Purchase{UserID: u.ID, PurchasedAt: s.Now().AddDate(0, 0, day-36), Source: "imported_invoice", TotalAmount: 100, Items: []domain.PurchaseItem{{ProductID: products[i].ID, Quantity: 1, UnitPrice: 100}}}
			if _, err = s.Purchase(ctx, p, fmt.Sprintf("seed-%d-%d", i, day)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return u, products
}
func TestEndToEndHTTPAndFeedback(t *testing.T) {
	s, store := setup(t)
	u, products := seed(t, s)
	handler := httpapi.API{Service: s}.Handler()
	request := func(method, path, body string, status int) []byte {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	raw := request("GET", "/users/"+u.ID+"/replenishment", "", 200)
	if !bytes.Contains(raw, []byte("low_replenishability")) {
		t.Fatal("television eligible")
	}
	var evaluation app.Evaluation
	raw = request("POST", "/users/"+u.ID+"/decisions/evaluate", "", 200)
	if err := json.Unmarshal(raw, &evaluation); err != nil {
		t.Fatal(err)
	}
	if len(evaluation.Suggestions) != 2 || len(evaluation.Decisions) != 3 {
		t.Fatalf("%+v", evaluation)
	}
	sg := evaluation.Suggestions[0]
	request("POST", "/suggestions/"+sg.ID+"/shown", "", 200)
	request("POST", "/suggestions/"+sg.ID+"/accept", "", 200)
	request("POST", "/suggestions/"+sg.ID+"/accept", "", 200)
	request("POST", "/suggestions/"+sg.ID+"/dismiss", "", 409)
	f, err := store.Facts(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	member := f.Membership[sg.Items[0].ProductID]
	if !member.InList || member.InCart {
		t.Fatalf("membership %+v", member)
	}
	counts := map[string]int{}
	for _, e := range f.Events {
		if e.DecisionID != nil && *e.DecisionID == sg.Items[0].DecisionID {
			counts[e.Type]++
		}
	}
	if counts["SUGGESTION_SHOWN"] != 1 || counts["SUGGESTION_ACCEPTED"] != 1 {
		t.Fatalf("duplicate feedback: %v", counts)
	}
	p := domain.Purchase{UserID: u.ID, PurchasedAt: s.Now(), Source: "app", TotalAmount: 200, Items: []domain.PurchaseItem{{ProductID: sg.Items[0].ProductID, Quantity: 2, UnitPrice: 100}}}
	first, err := s.Purchase(context.Background(), p, "new-purchase")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Purchase(context.Background(), p, "new-purchase")
	if err != nil || again.ID != first.ID {
		t.Fatal("not idempotent", err)
	}
	f, err = store.Facts(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	linked := 0
	for _, e := range f.Events {
		if e.Type == "PURCHASED" && e.DecisionID != nil && *e.DecisionID == sg.Items[0].DecisionID {
			linked++
		}
	}
	if linked != 1 {
		t.Fatalf("purchase attribution %d", linked)
	}
	raw = request("GET", "/users/"+u.ID+"/decision-history", "", 200)
	if !bytes.Contains(raw, []byte("decision_context_version")) {
		t.Fatal("missing audit context")
	}
	request("GET", "/users/"+u.ID+"/evaluation-metrics", "", 200)
	request("POST", "/users/"+u.ID+"/events", `{"product_id":"`+products[0].ID+`","event_type":"PURCHASED"}`, 400)
	request("POST", "/users", `{"timezone":"Asia/Bangkok","unexpected":1}`, 400)
	request("POST", "/users", `{"timezone":"bad/timezone"}`, 400)
	request("GET", "/users/not-a-uuid/replenishment", "", 400)
	request("GET", "/users/"+domain.NewID()+"/suggestions", "", 404)
}
func TestConcurrentEvaluationAndPolicy(t *testing.T) {
	s, store := setup(t)
	u, p := seed(t, s)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan app.Evaluation, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); e, err := s.Evaluate(ctx, u.ID); results <- e; errs <- err }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for e := range results {
		count += len(e.Suggestions)
	}
	if count != 2 {
		t.Fatalf("concurrent duplicates: %d", count)
	}
	f, err := store.Facts(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, sg := range f.Suggestions {
		if _, err = s.Feedback(ctx, sg.ID, "dismiss"); err != nil {
			t.Fatal(err)
		}
	}
	e, err := s.Evaluate(ctx, u.ID)
	if err != nil || len(e.Suggestions) != 0 {
		t.Fatal("cooldown bypass", err)
	}
	later := s.Now().Add(13 * time.Hour)
	s.Now = func() time.Time { return later }
	if err = s.SetMembership(ctx, u.ID, p[0].ID, domain.Membership{InCart: true}); err != nil {
		t.Fatal(err)
	}
	unavailable := false
	if _, err = s.UpdateProduct(ctx, p[1].ID, app.ProductPatch{Available: &unavailable}); err != nil {
		t.Fatal(err)
	}
	e, err = s.Evaluate(ctx, u.ID)
	if err != nil || len(e.Suggestions) != 0 {
		t.Fatal("membership/availability bypass", err)
	}
}

type fixedEngine struct {
	action decision.Action
	fail   bool
}

func (f fixedEngine) Decide(_ context.Context, c decision.DecisionContext) (decision.DecisionResult, error) {
	if f.fail {
		return decision.DecisionResult{}, errors.New("test provider unavailable")
	}
	return decision.DecisionResult{Action: f.action, Confidence: .9, Provider: "test", Model: "test", ContextVersion: c.Version}, nil
}
func TestShadowIsolationAndBundles(t *testing.T) {
	s, store := setup(t)
	u, _ := seed(t, s)
	ctx := context.Background()
	s.Shadow = fixedEngine{action: decision.AddToSmartListSuggestion}
	e, err := s.Evaluate(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Decisions) != 6 || len(e.Suggestions) != 2 {
		t.Fatalf("shadow affected actions: %+v", e)
	}
	prod := map[string]app.DecisionLog{}
	for _, l := range e.Decisions {
		if !l.Shadow {
			prod[l.ProductID] = l
			continue
		}
		if l.ExecutedAction != decision.DoNothing {
			t.Fatal("shadow executed")
		}
		a, _ := json.Marshal(l.Context)
		b, _ := json.Marshal(prod[l.ProductID].Context)
		if !bytes.Equal(a, b) {
			t.Fatal("shadow saw different context")
		}
	}
	f, err := store.Facts(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Membership) != 0 {
		t.Fatal("shadow changed list")
	}
	s.Shadow = fixedEngine{fail: true}
	e, err = s.Evaluate(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	failures := 0
	for _, l := range e.Decisions {
		if l.Shadow && l.Error != nil {
			failures++
		}
	}
	if failures != 3 {
		t.Fatalf("shadow failures not logged: %d", failures)
	}
	// A second user has its own histories but shares normalized products.
	u2, err := s.CreateUser(ctx, domain.User{Timezone: "UTC", SuggestionEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for id, history := range f.History {
		for i, o := range history {
			_, err = s.Purchase(ctx, domain.Purchase{UserID: u2.ID, PurchasedAt: o.PurchasedAt, Source: "app", TotalAmount: 1, Items: []domain.PurchaseItem{{ProductID: id, Quantity: 1, UnitPrice: 1}}}, fmt.Sprintf("%s-%d", id, i))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	s.Production = fixedEngine{action: decision.SuggestBundle}
	s.Shadow = nil
	e, err = s.Evaluate(ctx, u2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Suggestions) != 1 || len(e.Suggestions[0].Items) != 2 || e.Suggestions[0].Kind != "SUGGEST_BUNDLE" {
		t.Fatalf("invalid bundle: %+v", e)
	}
	if _, err = s.Feedback(ctx, e.Suggestions[0].ID, "accept"); err != nil {
		t.Fatal(err)
	}
	f2, err := store.Facts(ctx, u2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(f2.Membership) != 2 {
		t.Fatal("bundle acceptance did not add both items")
	}
}
func TestPurchaseRollbackAndConflict(t *testing.T) {
	s, store := setup(t)
	u, p := seed(t, s)
	ctx := context.Background()
	before, err := store.Facts(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	bad := domain.Purchase{UserID: u.ID, PurchasedAt: s.Now(), Source: "app", TotalAmount: 2, Items: []domain.PurchaseItem{{ProductID: p[0].ID, Quantity: 1, UnitPrice: 1}, {ProductID: domain.NewID(), Quantity: 1, UnitPrice: 1}}}
	if _, err = s.Purchase(ctx, bad, "bad"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expected FK failure, got %v", err)
	}
	after, err := store.Facts(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.PurchaseDates) != len(before.PurchaseDates) || len(after.Events) != len(before.Events) {
		t.Fatal("partial purchase committed")
	}
	good := domain.Purchase{UserID: u.ID, PurchasedAt: s.Now(), Source: "app", TotalAmount: 1, Items: []domain.PurchaseItem{{ProductID: p[0].ID, Quantity: 1, UnitPrice: 1}}}
	if _, err = s.Purchase(ctx, good, "key"); err != nil {
		t.Fatal(err)
	}
	good.Items[0].Quantity = 2
	good.TotalAmount = 2
	if _, err = s.Purchase(ctx, good, "key"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("changed retry not rejected", err)
	}
}

type failingStore struct{ app.Store }
type failingTx struct{ app.Transaction }

func (f failingStore) WithUser(ctx context.Context, id string, fn func(app.Transaction) error) error {
	return f.Store.WithUser(ctx, id, func(tx app.Transaction) error { return fn(failingTx{tx}) })
}
func (f failingTx) SaveSuggestion(context.Context, domain.Suggestion) error {
	return errors.New("injected persistence failure")
}
func TestSuggestionAndLogRollback(t *testing.T) {
	s, store := setup(t)
	u, _ := seed(t, s)
	s.Store = failingStore{store}
	if _, err := s.Evaluate(context.Background(), u.ID); err == nil {
		t.Fatal("expected failure")
	}
	logs, err := store.AllDecisions(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	f, err := store.Facts(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 0 || len(f.Suggestions) != 0 {
		t.Fatal("partial decision transaction committed")
	}
}
