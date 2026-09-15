.PHONY: run test build lint

run:
	go run ./cmd/server

test:
	go test ./... -cover

build:
	go build -o bin/shortlink ./cmd/server

lint:
	go vet ./...
