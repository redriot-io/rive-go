.PHONY: generate test build test-integration validate validate-wasm test-no-wasm conformance

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

# validate: generate all example .riv files then run structural validation.
validate:
	go run ./cmd/examples/
	GOTMPDIR=/app/workspace/tmp go test -v -timeout 60s ./test/validate/...

# validate-wasm: run all docs/preview/*.riv files through the Rive WASM runtime.
# Requires Node.js >= 20 and `npm ci` in tools/wasm-harness/ to have been run.
validate-wasm:
	cd tools/wasm-harness && npm ci
	node tools/wasm-harness/validate-all.js docs/preview/

# test-no-wasm: Go tests only, no WASM or browser dependency.
test-no-wasm:
	GOTMPDIR=/app/workspace/tmp go test ./...

# conformance: pull latest rive-runtime golden files, regenerate format contract
# and format rules, then run the full test suite.
# Requires network access (sparse-clones github.com/rive-app/rive-runtime).
conformance:
	@echo "Fetching rive-runtime golden files..."
	@rm -rf /tmp/rive-runtime
	@git clone --depth=1 --filter=blob:none --sparse \
		https://github.com/rive-app/rive-runtime /tmp/rive-runtime
	@cd /tmp/rive-runtime && git sparse-checkout set dev/defs tests/unit_tests/assets
	@cp /tmp/rive-runtime/tests/unit_tests/assets/*.riv rive/testdata/official/
	@echo "Regenerating format contract..."
	@go run ./cmd/rivtool analyze \
		--assets rive/testdata/official/ \
		--defs /tmp/rive-runtime/dev/defs/ \
		-o rive/format_contract.json
	@go generate ./rive/...
	@echo "Running tests..."
	@GOTMPDIR=/app/workspace/tmp go test ./...
	@echo "Done. Run 'git diff rive/' to inspect any format drift."
