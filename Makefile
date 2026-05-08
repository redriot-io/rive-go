.PHONY: generate test build

generate:
	go run ./cmd/rivegen -defs=internal/schema/defs -out=rive

test:
	GOTMPDIR=/app/workspace/tmp go test ./...

build:
	go build ./...
