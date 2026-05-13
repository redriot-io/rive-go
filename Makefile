.PHONY: generate gen-defaults test build test-integration validate validate-wasm test-no-wasm prove-contract prove-contract-bisect-demo conformance

generate:
	go run ./cmd/rivegen -defs=internal/schema/defs -out=rive
	$(MAKE) gen-defaults

# gen-defaults: prove all contract types then regenerate rive/gen_defaults.go.
gen-defaults:
	$(MAKE) prove-contract
	GOTMPDIR=$(CURDIR)/gotmp go run ./cmd/gen-defaults/ \
		--contract format_contract_proven.json \
		--out rive/gen_defaults.go \
		--package rive

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

# prove-contract: generate minimal .riv fixtures per object type and validate via WASM.
# Reads format_contract_proposed.json, writes format_contract_proven.json.
# All types in typeOrder must pass; exit 1 if none pass.
prove-contract:
	go run ./cmd/contract-prover/ \
		--proposed format_contract_proposed.json \
		--out format_contract_proven.json \
		--harness tools/wasm-harness/validate.js
	node tools/wasm-harness/validate-all.js testdata/prover/

# prove-contract-bisect-demo: same as prove-contract but forces Image into broken mode.
# Demonstrates property bisection: Image fails, bisection suggests blendModeValue=3.
# Output written to format_contract_proven_demo.json (not the canonical proven file).
prove-contract-bisect-demo:
	go run ./cmd/contract-prover/ \
		--proposed format_contract_proposed.json \
		--out format_contract_proven_demo.json \
		--harness tools/wasm-harness/validate.js \
		--force-fail Image

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
