package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"smartreplenish/internal/domain"
)

func (t *transaction) PurchaseByKey(ctx context.Context, key string) (*domain.Purchase, string, error) {
	var raw []byte
	var hash string
	err := t.q.QueryRow(ctx, `SELECT to_jsonb(p) || jsonb_build_object('items',(SELECT jsonb_agg(jsonb_build_object('product_id',i.product_id,'quantity',i.quantity,'unit_price',i.unit_price) ORDER BY i.product_id) FROM purchase_items i WHERE i.purchase_id=p.id)),payload_hash FROM purchases p WHERE user_id=$1 AND idempotency_key=$2`, t.user, key).Scan(&raw, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	var p domain.Purchase
	err = json.Unmarshal(raw, &p)
	return &p, hash, err
}
func (t *transaction) SavePurchase(ctx context.Context, p domain.Purchase, key, hash string) error {
	_, err := t.q.Exec(ctx, "INSERT INTO purchases VALUES($1,$2,$3,$4,$5,$6,$7)", p.ID, t.user, p.PurchasedAt, p.Source, p.TotalAmount, key, hash)
	if err != nil {
		return err
	}
	for _, i := range p.Items {
		if _, err = t.q.Exec(ctx, "INSERT INTO purchase_items VALUES($1,$2,$3,$4)", p.ID, i.ProductID, i.Quantity, i.UnitPrice); err != nil {
			return err
		}
	}
	return nil
}
func (t *transaction) SaveMembership(ctx context.Context, product string, m domain.Membership) error {
	_, err := t.q.Exec(ctx, `INSERT INTO user_product_state(user_id,product_id,in_cart,in_list,cart_quantity,auto_added) VALUES($1,$2,$3,$4,$5,$6)
 ON CONFLICT(user_id,product_id) DO UPDATE SET in_cart=EXCLUDED.in_cart,in_list=EXCLUDED.in_list,cart_quantity=EXCLUDED.cart_quantity,auto_added=EXCLUDED.auto_added`, t.user, product, m.InCart, m.InList, m.CartQuantity, m.AutoAdded)
	return err
}
func (t *transaction) SaveEvent(ctx context.Context, e domain.Event) error {
	if len(e.Metadata) == 0 {
		e.Metadata = json.RawMessage(`{}`)
	}
	_, err := t.q.Exec(ctx, `INSERT INTO user_events VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`, e.ID, t.user, e.Type, e.ProductID, e.DecisionID, e.SuggestionID, e.Metadata, e.CreatedAt)
	return err
}
func (t *transaction) SaveSuggestion(ctx context.Context, s domain.Suggestion) error {
	_, err := t.q.Exec(ctx, "INSERT INTO suggestions VALUES($1,$2,$3,$4,$5,$6,$7)", s.ID, t.user, s.Kind, s.Message, s.Section, s.Status, s.CreatedAt)
	if err != nil {
		return err
	}
	for _, i := range s.Items {
		if _, err = t.q.Exec(ctx, "INSERT INTO suggestion_items VALUES($1,$2,$3,$4)", s.ID, t.user, i.ProductID, i.DecisionID); err != nil {
			return err
		}
	}
	return nil
}
func (t *transaction) SetSuggestionStatus(ctx context.Context, id, status string) error {
	tag, err := t.q.Exec(ctx, "UPDATE suggestions SET status=$3 WHERE id=$1 AND user_id=$2", id, t.user, status)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}
