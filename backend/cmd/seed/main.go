package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"smartreplenish/internal/app"
	"smartreplenish/internal/config"
	"smartreplenish/internal/domain"
	"smartreplenish/internal/postgres"
	"smartreplenish/migrations"
	"time"
)

const userID = "00000000-0000-4000-8000-000000000001"

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, config.Env("DATABASE_URL", config.DefaultDatabaseURL))
	if err != nil {
		return err
	}
	defer pool.Close()
	if err = migrations.Apply(ctx, pool); err != nil {
		return err
	}
	store := &postgres.Store{Pool: pool}
	service := app.Service{Store: store}
	err = store.CreateUser(ctx, domain.User{ID: userID, Timezone: "Asia/Bangkok", SuggestionEnabled: true, CreatedAt: time.Now().UTC()})
	if err != nil && !errors.Is(err, domain.ErrConflict) {
		return err
	}
	f, err := store.Facts(ctx, userID)
	if err != nil {
		return err
	}
	anchor := f.User.CreatedAt
	samples := []struct {
		Product domain.Product
		Days    []int
	}{
		{domain.Product{ID: "00000000-0000-4000-8000-000000000101", SKU: "milk-001", Name: "Milk", Category: "milk", Unit: "carton", ReplenishableScore: .98, Available: true}, []int{1, 8, 15, 22, 30}},
		{domain.Product{ID: "00000000-0000-4000-8000-000000000102", SKU: "eggs-001", Name: "Eggs", Category: "eggs", Unit: "box", ReplenishableScore: .95, Available: true}, []int{1, 9, 17, 25}},
		{domain.Product{ID: "00000000-0000-4000-8000-000000000103", SKU: "tv-001", Name: "Television", Category: "electronics", Unit: "piece", ReplenishableScore: .01, Available: true}, []int{1}},
	}
	for _, sample := range samples {
		if err = store.CreateProduct(ctx, sample.Product); err != nil && !errors.Is(err, domain.ErrConflict) {
			return err
		}
		for _, day := range sample.Days {
			p := domain.Purchase{UserID: userID, PurchasedAt: anchor.AddDate(0, 0, day-36), Source: "imported_invoice", TotalAmount: 100, Items: []domain.PurchaseItem{{ProductID: sample.Product.ID, Quantity: 1, UnitPrice: 100}}}
			if _, err = service.Purchase(ctx, p, fmt.Sprintf("seed:%s:day:%d", sample.Product.SKU, day)); err != nil {
				return err
			}
		}
	}
	fmt.Printf("Seed ready. User: %s\nAnchor (day 36): %s\n", userID, anchor.Format(time.RFC3339))
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
