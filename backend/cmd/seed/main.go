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
		Product  domain.Product
		Days     []int
		Quantity int64
	}{
		{domain.Product{ID: "00000000-0000-4000-8000-000000000101", SKU: "milk-001", Name: "Milk", Category: "milk", Unit: "carton", ReplenishableScore: .98, Available: true}, []int{1, 8, 15, 22, 30}, 1},
		{domain.Product{ID: "00000000-0000-4000-8000-000000000102", SKU: "eggs-001", Name: "Eggs", Category: "eggs", Unit: "box", ReplenishableScore: .95, Available: true}, []int{1, 9, 17, 25}, 1},
		{domain.Product{ID: "00000000-0000-4000-8000-000000000103", SKU: "tv-001", Name: "Television", Category: "electronics", Unit: "piece", ReplenishableScore: .01, Available: true}, []int{1}, 1},
		// Water is usually bought two packs at a time, so a confirmed question adds two.
		{domain.Product{ID: "00000000-0000-4000-8000-000000000104", SKU: "water-001", Name: "Drinking water 6×1.5L", Category: "beverages", Brand: "Crystal", Unit: "pack", ReplenishableScore: .97, Available: true}, []int{2, 9, 16, 23, 31}, 2},
		// Catalogue-only products give the shopper app something to browse.
		{domain.Product{ID: "00000000-0000-4000-8000-000000000105", SKU: "bread-001", Name: "Wholewheat bread", Category: "bakery", Brand: "Farmhouse", Unit: "loaf", ReplenishableScore: .9, Available: true}, nil, 0},
		{domain.Product{ID: "00000000-0000-4000-8000-000000000106", SKU: "rice-001", Name: "Jasmine rice 5kg", Category: "pantry", Brand: "Hom Mali", Unit: "bag", ReplenishableScore: .85, Available: true}, nil, 0},
		{domain.Product{ID: "00000000-0000-4000-8000-000000000107", SKU: "coffee-001", Name: "Ground coffee", Category: "beverages", Brand: "Doi Chaang", Unit: "bag", ReplenishableScore: .85, Available: true}, nil, 0},
		{domain.Product{ID: "00000000-0000-4000-8000-000000000108", SKU: "detergent-001", Name: "Laundry detergent", Category: "household", Brand: "Breeze", Unit: "bottle", ReplenishableScore: .8, Available: true}, nil, 0},
		{domain.Product{ID: "00000000-0000-4000-8000-000000000109", SKU: "tissue-001", Name: "Toilet tissue 24 rolls", Category: "household", Brand: "Scott", Unit: "pack", ReplenishableScore: .9, Available: true}, nil, 0},
		{domain.Product{ID: "00000000-0000-4000-8000-000000000110", SKU: "shampoo-001", Name: "Shampoo", Category: "personal care", Brand: "Sunsilk", Unit: "bottle", ReplenishableScore: .75, Available: true}, nil, 0},
		{domain.Product{ID: "00000000-0000-4000-8000-000000000111", SKU: "banana-001", Name: "Bananas", Category: "fruit", Brand: "", Unit: "bunch", ReplenishableScore: .9, Available: true}, nil, 0},
		{domain.Product{ID: "00000000-0000-4000-8000-000000000112", SKU: "yogurt-001", Name: "Greek yogurt", Category: "dairy", Brand: "Dutchie", Unit: "cup", ReplenishableScore: .9, Available: false}, nil, 0},
	}
	for _, sample := range samples {
		if err = store.CreateProduct(ctx, sample.Product); err != nil && !errors.Is(err, domain.ErrConflict) {
			return err
		}
		for _, day := range sample.Days {
			quantity := max(1, sample.Quantity)
			p := domain.Purchase{UserID: userID, PurchasedAt: anchor.AddDate(0, 0, day-36), Source: "imported_invoice", TotalAmount: 100 * quantity, Items: []domain.PurchaseItem{{ProductID: sample.Product.ID, Quantity: quantity, UnitPrice: 100}}}
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
