package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"smartreplenish/internal/app"
	"smartreplenish/internal/domain"
)

type Store struct{ Pool *pgxpool.Pool }
type querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type transaction struct {
	q    pgx.Tx
	user string
}

func normalize(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "23505":
			return domain.ErrConflict
		case "23503", "23514", "22P02", "22003":
			return domain.ErrInvalid
		}
	}
	return err
}
func queryJSON[T any](ctx context.Context, q querier, sql string, args ...any) ([]T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, normalize(err)
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var raw []byte
		var v T
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, normalize(rows.Err())
}
func oneJSON[T any](ctx context.Context, q querier, sql string, args ...any) (T, error) {
	var v T
	var raw []byte
	err := q.QueryRow(ctx, sql, args...).Scan(&raw)
	if err != nil {
		return v, normalize(err)
	}
	err = json.Unmarshal(raw, &v)
	return v, err
}
func (s *Store) CreateUser(ctx context.Context, u domain.User) error {
	_, err := s.Pool.Exec(ctx, "INSERT INTO users VALUES($1,$2,$3,$4,$5)", u.ID, u.Timezone, u.NotificationEnabled, u.SuggestionEnabled, u.CreatedAt)
	return normalize(err)
}
func (s *Store) CreateProduct(ctx context.Context, p domain.Product) error {
	_, err := s.Pool.Exec(ctx, "INSERT INTO products VALUES($1,$2,$3,$4,$5,$6,$7,$8)", p.ID, p.SKU, p.Name, p.Category, p.Brand, p.Unit, p.ReplenishableScore, p.Available)
	return normalize(err)
}
func (s *Store) UpdateProduct(ctx context.Context, id string, p app.ProductPatch) (domain.Product, error) {
	return oneJSON[domain.Product](ctx, s.Pool, `UPDATE products p
 SET available=COALESCE($2,available),replenishable_score=COALESCE($3,replenishable_score)
 WHERE id=$1 RETURNING to_jsonb(p)`, id, p.Available, p.ReplenishableScore)
}
func (s *Store) WithUser(ctx context.Context, id string, fn func(app.Transaction) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var locked string
	if err = tx.QueryRow(ctx, "SELECT id::text FROM users WHERE id=$1 FOR UPDATE", id).Scan(&locked); err != nil {
		return normalize(err)
	}
	if err = fn(&transaction{q: tx, user: id}); err != nil {
		return normalize(err)
	}
	return normalize(tx.Commit(ctx))
}
func (s *Store) Facts(ctx context.Context, id string) (app.Facts, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return app.Facts{}, err
	}
	defer tx.Rollback(context.Background())
	f, err := loadFacts(ctx, tx, id, false)
	if err != nil {
		return f, err
	}
	return f, tx.Commit(ctx)
}
func (t *transaction) Facts(ctx context.Context) (app.Facts, error) {
	return loadFacts(ctx, t.q, t.user, true)
}
func (t *transaction) SaveUser(ctx context.Context, u domain.User) error {
	_, err := t.q.Exec(ctx, "UPDATE users SET timezone=$2,notification_enabled=$3,suggestion_enabled=$4 WHERE id=$1", t.user, u.Timezone, u.NotificationEnabled, u.SuggestionEnabled)
	return err
}

const suggestionSelect = `SELECT to_jsonb(s) || jsonb_build_object('items',COALESCE((SELECT jsonb_agg(jsonb_build_object('product_id',i.product_id,'decision_id',i.decision_id) ORDER BY i.product_id) FROM suggestion_items i WHERE i.suggestion_id=s.id),'[]'::jsonb)) FROM suggestions s `

func (s *Store) Suggestion(ctx context.Context, id string) (domain.Suggestion, error) {
	return oneJSON[domain.Suggestion](ctx, s.Pool, suggestionSelect+"WHERE s.id=$1", id)
}
func (s *Store) History(ctx context.Context, user string, limit, offset int) ([]app.DecisionLog, error) {
	return queryJSON[app.DecisionLog](ctx, s.Pool, "SELECT to_jsonb(d) FROM decision_logs d WHERE user_id=$1 ORDER BY created_at DESC,id LIMIT $2 OFFSET $3", user, limit, offset)
}
func (s *Store) AllDecisions(ctx context.Context, user string) ([]app.DecisionLog, error) {
	return queryJSON[app.DecisionLog](ctx, s.Pool, "SELECT to_jsonb(d) FROM decision_logs d WHERE user_id=$1 ORDER BY created_at,id", user)
}
func saveDecision(ctx context.Context, q querier, d app.DecisionLog) error {
	c, err := json.Marshal(d.Context)
	if err != nil {
		return err
	}
	r, err := json.Marshal(d.Result)
	if err != nil {
		return err
	}
	p, err := json.Marshal(d.Policy)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `INSERT INTO decision_logs(id,evaluation_id,user_id,product_id,engine,shadow,decision_context,decision_result,policy_result,executed_action,error,latency_ms,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, d.ID, d.EvaluationID, d.UserID, d.ProductID, d.Engine, d.Shadow, c, r, p, d.ExecutedAction, d.Error, d.LatencyMS, d.CreatedAt)
	return normalize(err)
}
func (t *transaction) SaveDecision(ctx context.Context, d app.DecisionLog) error {
	return saveDecision(ctx, t.q, d)
}
func (s *Store) SaveShadow(ctx context.Context, d app.DecisionLog) error {
	return saveDecision(ctx, s.Pool, d)
}

var _ app.Store = (*Store)(nil)
