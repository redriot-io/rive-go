.PHONY: generate test build test-integration

generate:
	go run ./cmd/rivegen -defs=internal/schema/defs -out=rive

test:
	GOTMPDIR=/app/workspace/tmp go test ./...

build:
	go build ./...

# Integration tests: validate .riv files against the official Rive WASM runtime
# via headless Chromium (Rod). Chromium is auto-downloaded on first run (~130MB
# cache at ~/.cache/rod/) or use a system installation (apk add chromium).
#
# Requires network access on first run (CDN for @rive-app/canvas + browser download).
# Subsequent runs are fully offline once ~/.cache/rod/ is populated.
test-integration:
	GOTMPDIR=/app/workspace/tmp go test -tags=integration -v -timeout 180s ./test/integration/...
