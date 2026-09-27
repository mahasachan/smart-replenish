package postgres

import (
	"context"
	"smartreplenish/internal/app"
	"smartreplenish/internal/domain"
	"time"
)

func loadFacts(ctx context.Context, q querier, user string, lock bool) (app.Facts, error) {
	f := app.Facts{Products: map[string]domain.Product{}, History: map[string][]domain.Observation{}, Membership: map[string]domain.Membership{}}
	var err error
	f.User, err = oneJSON[domain.User](ctx, q, "SELECT to_jsonb(u) FROM users u WHERE id=$1", user)
	if err != nil {
		return f, err
	}
	sql := `SELECT to_jsonb(p) FROM products p WHERE EXISTS(SELECT 1 FROM purchase_items i JOIN purchases b ON b.id=i.purchase_id WHERE b.user_id=$1 AND i.product_id=p.id) ORDER BY p.id`
	if lock {
		sql += " FOR SHARE OF p"
	}
	products, err := queryJSON[domain.Product](ctx, q, sql, user)
	if err != nil {
		return f, err
	}
	for _, p := range products {
		f.Products[p.ID] = p
	}
	rows, err := q.Query(ctx, `SELECT i.product_id::text,b.purchased_at,i.quantity FROM purchases b JOIN purchase_items i ON b.id=i.purchase_id WHERE b.user_id=$1 ORDER BY b.purchased_at,b.id`, user)
	if err != nil {
		return f, err
	}
	for rows.Next() {
		var id string
		var o domain.Observation
		if err = rows.Scan(&id, &o.PurchasedAt, &o.Quantity); err != nil {
			rows.Close()
			return f, err
		}
		f.History[id] = append(f.History[id], o)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return f, err
	}
	rows, err = q.Query(ctx, "SELECT purchased_at FROM purchases WHERE user_id=$1 ORDER BY purchased_at", user)
	if err != nil {
		return f, err
	}
	for rows.Next() {
		var t time.Time
		if err = rows.Scan(&t); err != nil {
			rows.Close()
			return f, err
		}
		f.PurchaseDates = append(f.PurchaseDates, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return f, err
	}
	rows, err = q.Query(ctx, "SELECT product_id::text,in_cart,in_list,cart_quantity,auto_added FROM user_product_state WHERE user_id=$1", user)
	if err != nil {
		return f, err
	}
	for rows.Next() {
		var id string
		var m domain.Membership
		if err = rows.Scan(&id, &m.InCart, &m.InList, &m.CartQuantity, &m.AutoAdded); err != nil {
			rows.Close()
			return f, err
		}
		f.Membership[id] = m
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return f, err
	}
	f.Suggestions, err = queryJSON[domain.Suggestion](ctx, q, suggestionSelect+"WHERE s.user_id=$1 ORDER BY s.created_at,s.id", user)
	if err != nil {
		return f, err
	}
	f.Events, err = queryJSON[domain.Event](ctx, q, "SELECT to_jsonb(e) FROM user_events e WHERE user_id=$1 ORDER BY created_at,id", user)
	return f, err
}
