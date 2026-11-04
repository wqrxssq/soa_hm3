.PHONY: generate fmt test up down e2e

generate:
	sh scripts/generate.sh

fmt:
	gofmt -w .

test: generate
	test -z "$$(gofmt -l .)"
	go test ./...

up:
	docker compose up --build

down:
	docker compose down -v

e2e:
	python3 scripts/e2e.py
