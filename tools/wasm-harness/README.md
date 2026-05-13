# WASM Validation Harness

Validates generated `.riv` files against the Rive WASM runtime (`@rive-app/canvas-advanced@2.37.6`) in Node.js without a browser.

## Setup

```sh
cd tools/wasm-harness
npm ci
```

## Usage

**Validate all files in a directory:**
```sh
node tools/wasm-harness/validate-all.js docs/preview/
```

**Validate a single file:**
```sh
node tools/wasm-harness/validate.js docs/preview/fromjson_hello_world.riv
```
Exit codes: `0`=pass, `1`=load error, `2`=render fail, `3`=harness error.

**Via Make:**
```sh
make validate-wasm      # validate docs/preview/*.riv via WASM
make test-no-wasm       # run Go tests without WASM validation
```

## How it works

Emscripten requires DOM and WebGL globals. The harness provides minimal stubs so the WASM module initialises in Node.js. The WASM binary is loaded from disk via `wasmBinary` option to avoid Node.js `fetch` URL issues. Each `.riv` file is loaded with `rive.load()` and considered passing if it returns a non-null file with at least one artboard.

## CI

GitHub Actions runs `make validate-wasm` on every push and pull request. See `.github/workflows/ci.yml`.
