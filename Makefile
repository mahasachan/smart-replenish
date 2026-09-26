.PHONY: db run seed test integration check frontend-install frontend frontend-build

db:
	docker compose up -d --wait

run:
	@set -a; if [ -f backend/.env ]; then . backend/.env; fi; set +a; cd backend && go run ./cmd/server

seed:
	@set -a; if [ -f backend/.env ]; then . backend/.env; fi; set +a; cd backend && go run ./cmd/seed

test:
	cd backend && go test ./...

integration:
	@set -a; if [ -f backend/.env ]; then . backend/.env; fi; set +a; cd backend && TEST_DATABASE_URL="$${DATABASE_URL:-postgres://smart:smart@localhost:55432/smartreplenish?sslmode=disable}" go test -race ./internal/postgres -count=1

check:
	cd backend && go vet ./... && go test -race ./...
	$(MAKE) -C frontend build

frontend-install:
	npm --prefix frontend ci

frontend:
	npm --prefix frontend run dev -- --host 127.0.0.1

frontend-build:
	npm --prefix frontend run build
