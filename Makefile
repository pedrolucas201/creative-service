run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

test:
	go test ./...

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v
