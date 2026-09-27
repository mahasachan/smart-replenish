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
	if sg.Kind != "ASK_IF_RUNNING_LOW" || member.InList || !member.InCart || member.CartQuantity != 1 || !member.AutoAdded {
		t.Fatalf("confirmed question did not add to cart: %s %+v", sg.Kind, member)
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
	if _, err = s.SetMembership(ctx, u.ID, p[0].ID, domain.Membership{InCart: true}); err != nil {
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
func TestRunningLowQuestionCartAndSnooze(t *testing.T) {
	s, store := setup(t)
	u, products := seed(t, s)
	ctx := context.Background()
	milk, eggs := products[0].ID, products[1].ID
	// Milk was mostly bought two at a time, so a confirmed question adds two units.
	for i, day := range []int{-92, -85, -78, -71, -64, -57} {
		p := domain.Purchase{UserID: u.ID, PurchasedAt: s.Now().AddDate(0, 0, day), Source: "receipt", TotalAmount: 200, Items: []domain.PurchaseItem{{ProductID: milk, Quantity: 2, UnitPrice: 100}}}
		if _, err := s.Purchase(ctx, p, fmt.Sprintf("double-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	handler := httpapi.API{Service: s}.Handler()
	request := func(method, path, body string, status int) []byte {
		t.Helper()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	var e app.Evaluation
	if err := json.Unmarshal(request("POST", "/users/"+u.ID+"/decisions/evaluate", "", 200), &e); err != nil {
		t.Fatal(err)
	}
	questions := map[string]domain.Suggestion{}
	for _, sg := range e.Suggestions {
		if sg.Kind != "ASK_IF_RUNNING_LOW" || sg.Status != "pending" {
			t.Fatalf("expected a pending running-low question: %+v", sg)
		}
		questions[sg.Items[0].ProductID] = sg
	}
	if len(questions) != 2 {
		t.Fatalf("questions: %+v", e.Suggestions)
	}
	request("POST", "/suggestions/"+questions[milk].ID+"/answer", `{}`, 400)
	request("POST", "/suggestions/"+questions[milk].ID+"/answer", `{"running_low":true}`, 200)
	request("POST", "/suggestions/"+questions[milk].ID+"/answer", `{"running_low":true}`, 200)
	request("POST", "/suggestions/"+questions[milk].ID+"/answer", `{"running_low":false}`, 409)
	request("POST", "/suggestions/"+questions[eggs].ID+"/answer", `{"running_low":false}`, 200)

	var cart []app.CartLine
	if err := json.Unmarshal(request("GET", "/users/"+u.ID+"/cart", "", 200), &cart); err != nil {
		t.Fatal(err)
	}
	if len(cart) != 1 || cart[0].ProductID != milk || !cart[0].InCart || cart[0].CartQuantity != 2 || !cart[0].AutoAdded || cart[0].InList {
		t.Fatalf("cart after yes/no: %+v", cart)
	}
	f, err := store.Facts(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	added := 0
	for _, ev := range f.Events {
		if ev.Type == "ADDED_TO_CART" && ev.DecisionID != nil && *ev.DecisionID == questions[milk].Items[0].DecisionID {
			added++
		}
		if ev.Type == "PURCHASED" && ev.CreatedAt.After(s.Now().Add(-time.Hour)) {
			t.Fatal("confirmation must never create a purchase")
		}
	}
	if added != 1 || len(f.PurchaseDates) != 16 {
		t.Fatalf("auto-add events %d, purchases %d", added, len(f.PurchaseDates))
	}

	// Manual quantity edits keep the marker; removing the item clears it.
	request("PUT", "/users/"+u.ID+"/products/"+milk+"/state", `{"in_cart":true,"cart_quantity":3}`, 200)
	request("PUT", "/users/"+u.ID+"/products/"+milk+"/state", `{"in_cart":false,"cart_quantity":3}`, 400)
	request("PUT", "/users/"+u.ID+"/products/"+milk+"/state", `{"in_cart":true,"cart_quantity":1000}`, 400)
	f, _ = store.Facts(ctx, u.ID)
	if m := f.Membership[milk]; m.CartQuantity != 3 || !m.AutoAdded {
		t.Fatalf("manual quantity: %+v", m)
	}

	// Eggs' median interval is eight days, so "not yet" snoozes them for four days.
	for _, hours := range []float64{13, 95} {
		later := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC).Add(time.Duration(hours * float64(time.Hour)))
		s.Now = func() time.Time { return later }
		e, err = s.Evaluate(ctx, u.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range e.Decisions {
			if d.ProductID == eggs && (d.Policy == nil || d.Policy.Reason != "running_low_snoozed") {
				t.Fatalf("eggs not snoozed at +%vh: %+v", hours, d.Policy)
			}
		}
		if len(e.Suggestions) != 0 {
			t.Fatalf("asked during snooze or while in cart: %+v", e.Suggestions)
		}
	}
	later := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC).Add(97 * time.Hour)
	s.Now = func() time.Time { return later }
	e, err = s.Evaluate(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Suggestions) != 1 || e.Suggestions[0].Items[0].ProductID != eggs {
		t.Fatalf("eggs should be asked again after the snooze: %+v", e.Suggestions)
	}

	var metrics app.Metrics
	if err = json.Unmarshal(request("GET", "/users/"+u.ID+"/evaluation-metrics", "", 200), &metrics); err != nil {
		t.Fatal(err)
	}
	if r := metrics.RunningLow; r.QuestionsShown != 2 || r.Confirmed.Numerator != 1 || r.Denied.Numerator != 1 {
		t.Fatalf("running-low metrics: %+v", r)
	}
	var catalogue []domain.Product
	if err = json.Unmarshal(request("GET", "/products?limit=2", "", 200), &catalogue); err != nil || len(catalogue) != 2 || catalogue[0].Name != "Eggs" {
		t.Fatalf("catalogue %+v %v", catalogue, err)
	}
	request("GET", "/products?limit=0", "", 400)
}
func TestMigrationUpgradesExistingCartRows(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run isolated PostgreSQL integration tests")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + strings.ReplaceAll(domain.NewID(), "-", "")
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	initial, err := os.ReadFile("../../migrations/001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	user, product := domain.NewID(), domain.NewID()
	for _, sql := range []string{string(initial),
		"CREATE TABLE schema_migrations (name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now()); INSERT INTO schema_migrations(name) VALUES('001_initial.sql')",
		"INSERT INTO users VALUES('" + user + "','UTC',false,true,now())",
		"INSERT INTO products VALUES('" + product + "','sku','Soap','home','','bar',0.9,true)",
		"INSERT INTO user_product_state VALUES('" + user + "','" + product + "',true,false)"} {
		if _, err = pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	if err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var quantity int64
	if err = pool.QueryRow(ctx, "SELECT cart_quantity FROM user_product_state").Scan(&quantity); err != nil || quantity != 1 {
		t.Fatalf("existing cart row quantity %d %v", quantity, err)
	}
}
